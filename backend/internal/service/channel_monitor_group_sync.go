package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// 按分组自动同步渠道监控：每个启用分组下的每个可用 API Key 账号对应一条监控，
// 直接用该账号的上游地址和凭证做 GET /v1/models 目录探测，模型清单取账号白名单。
// 这样渠道状态页展示的就是用户实际能选的分组和模型，而且不需要把上游 key
// 再手工录入一遍；白名单、分组状态变化会在下一轮同步时跟上。
const (
	monitorGroupSyncIntervalSeconds = 1800
	monitorGroupSyncJitterSeconds   = 60
	monitorGroupSyncPeriod          = 10 * time.Minute
	monitorGroupSyncStartupDelay    = 30 * time.Second
	monitorGroupSyncTimeout         = 2 * time.Minute
	// 手工监控常年只有十几条，一页取完即可。
	monitorGroupSyncListPageSize = 1000
)

// ChannelMonitorGroupSyncResult 一次分组同步的结果汇总。
type ChannelMonitorGroupSyncResult struct {
	Created   int
	Updated   int
	Disabled  int
	Unchanged int
	// Skipped 记录没能生成监控的分组/账号及原因，供管理员排查。
	Skipped []string
}

// channelMonitorGroupSource 同步任务读取分组、账号、渠道的依赖。
type channelMonitorGroupSource struct {
	groupRepo      GroupRepository
	accountRepo    AccountRepository
	channelService *ChannelService
}

// monitorGroupSyncTarget 一条监控在同步后应有的状态。
type monitorGroupSyncTarget struct {
	GroupID      int64
	AccountID    int64
	Name         string
	GroupName    string
	Provider     string
	Endpoint     string
	APIKey       string
	Models       []string // 已按优先展示顺序排好，Models[0] 为默认主模型
	ExtraHeaders map[string]string
}

type monitorGroupSyncKey struct {
	groupID   int64
	accountID int64
}

// monitorGroupSyncUpdate 对已有监控的一次更新。
type monitorGroupSyncUpdate struct {
	ID     int64
	Params ChannelMonitorUpdateParams
}

// monitorGroupSyncPlan 由现状和目标算出的变更集合，执行阶段按顺序落库。
type monitorGroupSyncPlan struct {
	Creates   []ChannelMonitorCreateParams
	Updates   []monitorGroupSyncUpdate
	Disables  []int64
	Unchanged int
}

// SetGroupSource 注入分组同步所需的依赖；未注入时 SyncFromGroups 返回错误。
func (s *ChannelMonitorService) SetGroupSource(groupRepo GroupRepository, accountRepo AccountRepository, channelService *ChannelService) {
	if groupRepo == nil || accountRepo == nil {
		return
	}
	s.groupSource = &channelMonitorGroupSource{
		groupRepo:      groupRepo,
		accountRepo:    accountRepo,
		channelService: channelService,
	}
}

// SyncFromGroups 按当前启用的分组和账号白名单创建、更新或停用监控。
// 只动由同步任务创建的监控（SourceGroupID/SourceAccountID 非空），手工监控不受影响。
func (s *ChannelMonitorService) SyncFromGroups(ctx context.Context) (*ChannelMonitorGroupSyncResult, error) {
	if s.groupSource == nil {
		return nil, ErrChannelMonitorGroupSyncUnavailable
	}
	s.groupSyncMu.Lock()
	defer s.groupSyncMu.Unlock()

	targets, skipped, err := s.groupSource.loadTargets(ctx)
	if err != nil {
		return nil, err
	}
	existing, _, err := s.List(ctx, ChannelMonitorListParams{Page: 1, PageSize: monitorGroupSyncListPageSize})
	if err != nil {
		return nil, fmt.Errorf("list channel monitors: %w", err)
	}

	plan := planMonitorGroupSync(existing, targets)
	result := &ChannelMonitorGroupSyncResult{Unchanged: plan.Unchanged, Skipped: skipped}
	for _, p := range plan.Creates {
		if _, err := s.Create(ctx, p); err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("%s：创建监控失败：%v", p.Name, err))
			continue
		}
		result.Created++
	}
	for _, u := range plan.Updates {
		if _, err := s.Update(ctx, u.ID, u.Params); err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("监控 #%d：更新失败：%v", u.ID, err))
			continue
		}
		result.Updated++
	}
	disabled := false
	for _, id := range plan.Disables {
		if _, err := s.Update(ctx, id, ChannelMonitorUpdateParams{Enabled: &disabled}); err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("监控 #%d：停用失败：%v", id, err))
			continue
		}
		result.Disabled++
	}
	return result, nil
}

// runGroupSyncOnce 供调度器周期调用：失败只记日志。
func (s *ChannelMonitorService) runGroupSyncOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, monitorGroupSyncTimeout)
	defer cancel()
	res, err := s.SyncFromGroups(ctx)
	if err != nil {
		slog.Error("channel_monitor: group sync failed", "error", err)
		return
	}
	if res.Created+res.Updated+res.Disabled > 0 || len(res.Skipped) > 0 {
		slog.Info("channel_monitor: group sync done",
			"created", res.Created, "updated", res.Updated, "disabled", res.Disabled,
			"unchanged", res.Unchanged, "skipped", res.Skipped)
	}
}

func (src *channelMonitorGroupSource) loadTargets(ctx context.Context) ([]monitorGroupSyncTarget, []string, error) {
	groups, err := src.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list active groups: %w", err)
	}
	accountsByGroup := make(map[int64][]Account, len(groups))
	channelByGroup := make(map[int64]*Channel, len(groups))
	for _, g := range groups {
		accounts, err := src.accountRepo.ListByGroup(ctx, g.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("list accounts of group %d: %w", g.ID, err)
		}
		accountsByGroup[g.ID] = accounts
		if src.channelService != nil {
			ch, err := src.channelService.GetChannelForGroup(ctx, g.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("load channel of group %d: %w", g.ID, err)
			}
			channelByGroup[g.ID] = ch
		}
	}
	targets, skipped := buildMonitorGroupSyncTargets(groups, accountsByGroup, channelByGroup)
	return targets, skipped, nil
}

// buildMonitorGroupSyncTargets 把分组、账号、渠道换算成每条监控的目标状态。
// 分组只绑一个可用账号时监控名就是分组名；绑多个时加「（线路N）」区分。
func buildMonitorGroupSyncTargets(groups []Group, accountsByGroup map[int64][]Account, channelByGroup map[int64]*Channel) ([]monitorGroupSyncTarget, []string) {
	var targets []monitorGroupSyncTarget
	var skipped []string
	for _, g := range groups {
		var groupTargets []monitorGroupSyncTarget
		for i := range accountsByGroup[g.ID] {
			a := &accountsByGroup[g.ID][i]
			t, reason := monitorGroupSyncTargetFor(&g, a, channelByGroup[g.ID])
			if reason != "" {
				skipped = append(skipped, fmt.Sprintf("%s / 账号 #%d：%s", g.Name, a.ID, reason))
				continue
			}
			groupTargets = append(groupTargets, t)
		}
		if len(groupTargets) > 1 {
			for i := range groupTargets {
				groupTargets[i].Name = truncateMonitorName(fmt.Sprintf("%s（线路%d）", g.Name, i+1))
			}
		}
		targets = append(targets, groupTargets...)
	}
	return targets, skipped
}

func monitorGroupSyncTargetFor(g *Group, a *Account, ch *Channel) (monitorGroupSyncTarget, string) {
	if a.Type != AccountTypeAPIKey {
		return monitorGroupSyncTarget{}, "不是 API Key 账号"
	}
	if a.ParentAccountID != nil {
		return monitorGroupSyncTarget{}, "影子账号，跟随母账号监控"
	}
	provider := monitorProviderForPlatform(a.Platform)
	if provider == "" {
		return monitorGroupSyncTarget{}, fmt.Sprintf("平台 %s 不支持目录探测", a.Platform)
	}
	apiKey := strings.TrimSpace(a.GetCredential("api_key"))
	if apiKey == "" {
		return monitorGroupSyncTarget{}, "未填写 API Key"
	}
	endpoint, ok := monitorEndpointFromBaseURL(a.GetCredential("base_url"))
	if !ok {
		return monitorGroupSyncTarget{}, "base_url 为空或不是 https 根地址"
	}
	models := monitorModelsForAccount(a, ch)
	if len(models) == 0 {
		return monitorGroupSyncTarget{}, "白名单与渠道定价没有可用模型"
	}
	t := monitorGroupSyncTarget{
		GroupID:   g.ID,
		AccountID: a.ID,
		Name:      truncateMonitorName(g.Name),
		GroupName: truncateMonitorName(g.Name),
		Provider:  provider,
		Endpoint:  endpoint,
		APIKey:    apiKey,
		Models:    models,
	}
	// 火神等上游会透传 UA，默认 UA 可能被拒；与账号实际请求保持一致。
	if ua := strings.TrimSpace(a.GetCredential("user_agent")); ua != "" {
		t.ExtraHeaders = map[string]string{"User-Agent": ua}
	}
	return t, ""
}

func monitorProviderForPlatform(platform string) string {
	switch platform {
	case PlatformOpenAI:
		return MonitorProviderOpenAI
	case PlatformAnthropic:
		return MonitorProviderAnthropic
	case PlatformGemini:
		return MonitorProviderGemini
	case PlatformGrok:
		return MonitorProviderGrok
	default:
		return ""
	}
}

// monitorEndpointFromBaseURL 把账号 base_url 归一成监控要求的 https origin。
// 常见的 ".../v1" 写法去掉后缀，否则 checker 会拼出 /v1/v1/models。
func monitorEndpointFromBaseURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	if path != "" {
		return "", false
	}
	return "https://" + u.Host, true
}

// monitorModelsForAccount 取账号白名单；渠道开了「仅允许已定价模型」时再与定价取交集，
// 因为只有交集里的模型用户才调得通。白名单为空时退回渠道定价清单。
// 结果按名称倒序，让同一系列的新版本（如 kimi-k3、gpt-6-astra）排在前面当主模型。
func monitorModelsForAccount(a *Account, ch *Channel) []string {
	whitelist := accountWhitelistModels(a)
	priced := channelPricedModels(ch)
	var models []string
	switch {
	case len(whitelist) > 0 && ch != nil && ch.RestrictModels && len(priced) > 0:
		for _, m := range whitelist {
			if _, ok := priced[m]; ok {
				models = append(models, m)
			}
		}
	case len(whitelist) > 0:
		models = whitelist
	default:
		for m := range priced {
			models = append(models, m)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(models)))
	return models
}

func accountWhitelistModels(a *Account) []string {
	if a == nil || a.Credentials == nil {
		return nil
	}
	var keys []string
	switch raw := a.Credentials["model_mapping"].(type) {
	case map[string]any:
		for k := range raw {
			keys = append(keys, k)
		}
	case map[string]string:
		for k := range raw {
			keys = append(keys, k)
		}
	}
	return usableModelNames(keys)
}

func channelPricedModels(ch *Channel) map[string]struct{} {
	if ch == nil {
		return nil
	}
	out := make(map[string]struct{})
	for _, p := range ch.ModelPricing {
		for _, m := range usableModelNames(p.Models) {
			out[m] = struct{}{}
		}
	}
	return out
}

// usableModelNames 去掉空串和通配符条目：目录探测只能核对具体模型名。
func usableModelNames(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" || strings.Contains(m, "*") {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

func truncateMonitorName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= maxChannelMonitorNameRunes {
		return name
	}
	return string([]rune(name)[:maxChannelMonitorNameRunes])
}

// planMonitorGroupSync 对比现有监控与目标，算出需要新建、更新、停用的监控。
// existing 中的 APIKey 须已解密。同一来源出现多条监控时保留 ID 最小的一条，其余停用。
func planMonitorGroupSync(existing []*ChannelMonitor, targets []monitorGroupSyncTarget) monitorGroupSyncPlan {
	synced := make([]*ChannelMonitor, 0, len(existing))
	for _, m := range existing {
		if m.IsGroupSynced() {
			synced = append(synced, m)
		}
	}
	sort.Slice(synced, func(i, j int) bool { return synced[i].ID < synced[j].ID })

	byKey := make(map[monitorGroupSyncKey]*ChannelMonitor, len(synced))
	var plan monitorGroupSyncPlan
	for _, m := range synced {
		key := monitorGroupSyncKey{groupID: *m.SourceGroupID, accountID: *m.SourceAccountID}
		if _, dup := byKey[key]; dup {
			if m.Enabled {
				plan.Disables = append(plan.Disables, m.ID)
			}
			continue
		}
		byKey[key] = m
	}

	for _, t := range targets {
		key := monitorGroupSyncKey{groupID: t.GroupID, accountID: t.AccountID}
		m := byKey[key]
		if m == nil {
			plan.Creates = append(plan.Creates, monitorGroupSyncCreateParams(t))
			continue
		}
		delete(byKey, key)
		params, changed := monitorGroupSyncUpdateParams(m, t)
		if !changed {
			plan.Unchanged++
			continue
		}
		plan.Updates = append(plan.Updates, monitorGroupSyncUpdate{ID: m.ID, Params: params})
	}

	stale := make([]*ChannelMonitor, 0, len(byKey))
	for _, m := range byKey {
		stale = append(stale, m)
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].ID < stale[j].ID })
	for _, m := range stale {
		if m.Enabled {
			plan.Disables = append(plan.Disables, m.ID)
		}
	}
	return plan
}

func monitorGroupSyncCreateParams(t monitorGroupSyncTarget) ChannelMonitorCreateParams {
	groupID, accountID := t.GroupID, t.AccountID
	return ChannelMonitorCreateParams{
		Name:             t.Name,
		Provider:         t.Provider,
		APIMode:          MonitorAPIModeModels,
		Endpoint:         t.Endpoint,
		APIKey:           t.APIKey,
		PrimaryModel:     t.Models[0],
		ExtraModels:      append([]string{}, t.Models[1:]...),
		GroupName:        t.GroupName,
		Enabled:          true,
		IntervalSeconds:  monitorGroupSyncIntervalSeconds,
		JitterSeconds:    monitorGroupSyncJitterSeconds,
		ExtraHeaders:     cloneMonitorHeaders(t.ExtraHeaders),
		BodyOverrideMode: MonitorBodyOverrideModeOff,
		SourceGroupID:    &groupID,
		SourceAccountID:  &accountID,
	}
}

// monitorGroupSyncUpdateParams 只填需要变化的字段。管理员手工改过的主模型只要仍在
// 白名单里就保留；间隔、抖动也不覆盖。
func monitorGroupSyncUpdateParams(m *ChannelMonitor, t monitorGroupSyncTarget) (ChannelMonitorUpdateParams, bool) {
	var p ChannelMonitorUpdateParams
	changed := false
	setString := func(dst **string, cur, want string) {
		if cur != want {
			v := want
			*dst = &v
			changed = true
		}
	}
	setString(&p.Name, m.Name, t.Name)
	setString(&p.GroupName, m.GroupName, t.GroupName)
	setString(&p.Provider, m.Provider, t.Provider)
	setString(&p.Endpoint, m.Endpoint, t.Endpoint)
	setString(&p.APIMode, m.APIMode, MonitorAPIModeModels)
	if m.APIKeyDecryptFailed || m.APIKey != t.APIKey {
		v := t.APIKey
		p.APIKey = &v
		changed = true
	}

	primary := t.Models[0]
	for _, model := range t.Models {
		if model == m.PrimaryModel {
			primary = m.PrimaryModel
			break
		}
	}
	extras := make([]string, 0, len(t.Models)-1)
	for _, model := range t.Models {
		if model != primary {
			extras = append(extras, model)
		}
	}
	setString(&p.PrimaryModel, m.PrimaryModel, primary)
	if !equalStringSlices(m.ExtraModels, extras) {
		p.ExtraModels = &extras
		changed = true
	}

	wantHeaders := cloneMonitorHeaders(t.ExtraHeaders)
	if !equalStringMaps(m.ExtraHeaders, wantHeaders) {
		p.ExtraHeaders = &wantHeaders
		changed = true
	}
	if !m.Enabled {
		enabled := true
		p.Enabled = &enabled
		changed = true
	}
	return p, changed
}

func cloneMonitorHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
