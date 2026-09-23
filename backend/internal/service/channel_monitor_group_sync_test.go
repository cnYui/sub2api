//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func syncTestAccount(id int64, mapping map[string]any, extra map[string]any) Account {
	creds := map[string]any{
		"base_url": "https://api.example.com",
		"api_key":  "sk-upstream",
	}
	if mapping != nil {
		creds["model_mapping"] = mapping
	}
	for k, v := range extra {
		creds[k] = v
	}
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: creds}
}

func TestBuildMonitorGroupSyncTargets_WhitelistIntersectsRestrictedChannel(t *testing.T) {
	groups := []Group{{ID: 82, Name: "GLM0.4倍率（日常二）"}}
	accounts := map[int64][]Account{
		82: {syncTestAccount(1176, map[string]any{
			"glm-5.3-flash": "glm-5.3-flash",
			"glm-5.3":       "glm-5.3", // 白名单里有、渠道没定价 → 用户调不通，不应展示
		}, map[string]any{"user_agent": "OpenAI/Python 1.109.1"})},
	}
	channels := map[int64]*Channel{
		82: {RestrictModels: true, ModelPricing: []ChannelModelPricing{{Models: []string{"glm-5.3-flash"}}}},
	}

	targets, skipped := buildMonitorGroupSyncTargets(groups, accounts, channels)

	require.Empty(t, skipped)
	require.Len(t, targets, 1)
	got := targets[0]
	require.Equal(t, int64(82), got.GroupID)
	require.Equal(t, int64(1176), got.AccountID)
	require.Equal(t, "GLM0.4倍率（日常二）", got.Name)
	require.Equal(t, "GLM0.4倍率（日常二）", got.GroupName)
	require.Equal(t, MonitorProviderOpenAI, got.Provider)
	require.Equal(t, "https://api.example.com", got.Endpoint)
	require.Equal(t, "sk-upstream", got.APIKey)
	require.Equal(t, []string{"glm-5.3-flash"}, got.Models)
	require.Equal(t, map[string]string{"User-Agent": "OpenAI/Python 1.109.1"}, got.ExtraHeaders)
}

func TestBuildMonitorGroupSyncTargets_ModelOrderAndFallbacks(t *testing.T) {
	groups := []Group{{ID: 7, Name: "Kimi1倍率"}, {ID: 12, Name: "GPT生图1倍率"}}
	accounts := map[int64][]Account{
		7: {syncTestAccount(5, map[string]any{
			"kimi-k2.5": "kimi-k2.5",
			"kimi-k3":   "kimi-k3",
			"kimi-k2.6": "kimi-k2.6",
			"gpt-*":     "gpt-5.5", // 通配符无法用目录核对
		}, nil)},
		// 白名单为空时退回渠道定价清单
		12: {syncTestAccount(1131, nil, nil)},
	}
	channels := map[int64]*Channel{
		12: {ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-image-2"}}}},
	}

	targets, skipped := buildMonitorGroupSyncTargets(groups, accounts, channels)

	require.Empty(t, skipped)
	require.Len(t, targets, 2)
	require.Equal(t, []string{"kimi-k3", "kimi-k2.6", "kimi-k2.5"}, targets[0].Models)
	require.Nil(t, targets[0].ExtraHeaders)
	require.Equal(t, []string{"gpt-image-2"}, targets[1].Models)
}

func TestBuildMonitorGroupSyncTargets_SkipsUnusableAccountsAndNamesLines(t *testing.T) {
	oauth := syncTestAccount(3, map[string]any{"gpt-5.5": "gpt-5.5"}, nil)
	oauth.Type = AccountTypeOAuth
	noKey := syncTestAccount(4, map[string]any{"gpt-5.5": "gpt-5.5"}, nil)
	delete(noKey.Credentials, "api_key")
	badURL := syncTestAccount(6, map[string]any{"gpt-5.5": "gpt-5.5"}, map[string]any{"base_url": "http://plain.example.com"})
	withV1 := syncTestAccount(8, map[string]any{"gpt-5.5": "gpt-5.5"}, map[string]any{"base_url": "https://relay.example.com/v1/"})
	second := syncTestAccount(9, map[string]any{"gpt-5.5": "gpt-5.5"}, nil)

	groups := []Group{{ID: 10, Name: "GPT0.35倍率（优质）"}}
	accounts := map[int64][]Account{10: {oauth, noKey, badURL, withV1, second}}

	targets, skipped := buildMonitorGroupSyncTargets(groups, accounts, nil)

	require.Len(t, skipped, 3)
	require.Len(t, targets, 2)
	require.Equal(t, "GPT0.35倍率（优质）（线路1）", targets[0].Name)
	require.Equal(t, "https://relay.example.com", targets[0].Endpoint)
	require.Equal(t, "GPT0.35倍率（优质）（线路2）", targets[1].Name)
	require.Equal(t, "GPT0.35倍率（优质）", targets[1].GroupName)
}

func TestPlanMonitorGroupSync(t *testing.T) {
	gid := func(v int64) *int64 { return &v }
	target := func(groupID, accountID int64, models ...string) monitorGroupSyncTarget {
		return monitorGroupSyncTarget{
			GroupID: groupID, AccountID: accountID,
			Name: "G", GroupName: "G", Provider: MonitorProviderOpenAI,
			Endpoint: "https://api.example.com", APIKey: "sk-new", Models: models,
		}
	}
	synced := func(id, groupID, accountID int64, primary string, extras ...string) *ChannelMonitor {
		return &ChannelMonitor{
			ID: id, Name: "G", GroupName: "G", Provider: MonitorProviderOpenAI, APIMode: MonitorAPIModeModels,
			Endpoint: "https://api.example.com", APIKey: "sk-new", PrimaryModel: primary, ExtraModels: extras,
			ExtraHeaders: map[string]string{}, Enabled: true,
			SourceGroupID: gid(groupID), SourceAccountID: gid(accountID),
		}
	}

	manual := &ChannelMonitor{ID: 1, Name: "手工监控", Enabled: true}
	unchanged := synced(20, 81, 1175, "deepseek-v4.1-flash")
	// 管理员把主模型改成了 k2.6，且仍在白名单里 → 保留；key 轮换了 → 更新
	rotated := synced(21, 7, 5, "kimi-k2.6", "kimi-k3")
	rotated.APIKey = "sk-old"
	// 分组被停用后又恢复
	reenabled := synced(22, 76, 1173, "glm-5.3", "glm-5.3-flash")
	reenabled.Enabled = false
	stale := synced(23, 77, 1170, "deepseek-v4-flash")
	staleAlreadyOff := synced(24, 78, 1171, "glm-5.2")
	staleAlreadyOff.Enabled = false
	duplicate := synced(25, 81, 1175, "deepseek-v4.1-flash")

	existing := []*ChannelMonitor{duplicate, stale, manual, rotated, unchanged, reenabled, staleAlreadyOff}
	targets := []monitorGroupSyncTarget{
		target(81, 1175, "deepseek-v4.1-flash"),
		target(7, 5, "kimi-k3", "kimi-k2.6"),
		target(76, 1173, "glm-5.3-flash", "glm-5.3"),
		target(83, 1177, "kimi-k2.6"),
	}

	plan := planMonitorGroupSync(existing, targets)

	require.Equal(t, 1, plan.Unchanged)
	require.Len(t, plan.Creates, 1)
	create := plan.Creates[0]
	require.Equal(t, int64(83), *create.SourceGroupID)
	require.Equal(t, int64(1177), *create.SourceAccountID)
	require.Equal(t, "kimi-k2.6", create.PrimaryModel)
	require.Empty(t, create.ExtraModels)
	require.Equal(t, monitorGroupSyncIntervalSeconds, create.IntervalSeconds)
	require.Equal(t, MonitorAPIModeModels, create.APIMode)
	require.True(t, create.Enabled)

	require.Len(t, plan.Updates, 2)
	byID := map[int64]ChannelMonitorUpdateParams{}
	for _, u := range plan.Updates {
		byID[u.ID] = u.Params
	}
	rotatedParams := byID[21]
	require.NotNil(t, rotatedParams.APIKey)
	require.Equal(t, "sk-new", *rotatedParams.APIKey)
	require.Nil(t, rotatedParams.PrimaryModel, "仍在白名单里的手选主模型应保留")
	require.Nil(t, rotatedParams.ExtraModels)
	require.Nil(t, rotatedParams.Enabled)

	reenabledParams := byID[22]
	require.NotNil(t, reenabledParams.Enabled)
	require.True(t, *reenabledParams.Enabled)
	require.Nil(t, reenabledParams.APIKey)

	// 23 来源分组不在目标里 → 停用；25 与 20 同来源 → 停用 ID 较大的重复项；24 已停用不重复处理；手工监控不动
	require.Equal(t, []int64{25, 23}, plan.Disables)
}

func TestPlanMonitorGroupSync_PrimaryDroppedFromWhitelist(t *testing.T) {
	groupID, accountID := int64(7), int64(5)
	m := &ChannelMonitor{
		ID: 30, Name: "Kimi1倍率", GroupName: "Kimi1倍率", Provider: MonitorProviderOpenAI, APIMode: MonitorAPIModeModels,
		Endpoint: "https://api.example.com", APIKey: "sk", PrimaryModel: "kimi-k2.5", ExtraModels: []string{"kimi-k3"},
		Enabled: true, SourceGroupID: &groupID, SourceAccountID: &accountID,
	}
	target := monitorGroupSyncTarget{
		GroupID: 7, AccountID: 5, Name: "Kimi1倍率", GroupName: "Kimi1倍率", Provider: MonitorProviderOpenAI,
		Endpoint: "https://api.example.com", APIKey: "sk", Models: []string{"kimi-k3"},
	}

	plan := planMonitorGroupSync([]*ChannelMonitor{m}, []monitorGroupSyncTarget{target})

	require.Len(t, plan.Updates, 1)
	p := plan.Updates[0].Params
	require.NotNil(t, p.PrimaryModel)
	require.Equal(t, "kimi-k3", *p.PrimaryModel)
	require.NotNil(t, p.ExtraModels)
	require.Empty(t, *p.ExtraModels)
}

func TestSyncFromGroups_RequiresGroupSource(t *testing.T) {
	svc := NewChannelMonitorService(&duplicateChannelMonitorRepoStub{}, &duplicateChannelMonitorEncryptor{})
	_, err := svc.SyncFromGroups(context.Background())
	require.ErrorIs(t, err, ErrChannelMonitorGroupSyncUnavailable)
}
