package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcard"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 兑换卡：管理员把兑换码做成站长设计的 3D 卡片（黑色版 / 白色版），通过 /card/<token> 链接发给用户。
//
// 卡面分两部分：
//   - 与兑换码相关的文字（面值、套餐、用量、有效期、编号）存在每张卡的 content 里；
//   - 「主理人」信息、兑换步骤和两个二维码是全站共用的 profile，存在 settings 里，
//     改一次所有已发出的卡片都会更新（微信群二维码约 7 天过期，必须能统一换掉）。

const (
	RedeemCardThemeDark  = "dark"
	RedeemCardThemeLight = "light"

	SettingKeyRedeemCardProfile = "redeem_card_profile"

	redeemCardTextMaxRunes    = 64
	redeemCardSerialMaxRunes  = 16
	redeemCardMaxOwnerLines   = 4
	redeemCardMaxSteps        = 5
	redeemCardDefaultHeatSeed = 7
	redeemCardMaxHeatSeed     = 999
	// 二维码以 data URL 存进 settings，限制解码后大小，避免把整张高清图塞进设置表。
	redeemCardImageMaxBytes = 512 * 1024
)

var (
	ErrRedeemCardNotFound     = infraerrors.NotFound("REDEEM_CARD_NOT_FOUND", "redeem card not found")
	ErrRedeemCardCodeNotFound = infraerrors.NotFound("REDEEM_CARD_CODE_NOT_FOUND", "redeem code not found")

	redeemCardTokenPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	redeemCardImagePattern = regexp.MustCompile(`^data:image/(png|jpeg|webp);base64,([A-Za-z0-9+/=]+)$`)
)

// RedeemCardContent 是卡面上与兑换码相关的文字，全部由管理员填写（页面会按兑换码自动预填）。
type RedeemCardContent struct {
	Amount      string `json:"amount"`
	Plan        string `json:"plan"`
	Tokens      string `json:"tokens"`
	ValidUntil  string `json:"valid_until"`
	Serial      string `json:"serial"`
	HeatmapSeed int    `json:"heatmap_seed"`
}

// RedeemCardProfile 是所有卡片共用的卡面信息。二维码图片为空时前端使用站点默认图（/email/qr-*.png）。
type RedeemCardProfile struct {
	OwnerLabel     string   `json:"owner_label"`
	OwnerName      string   `json:"owner_name"`
	OwnerLines     []string `json:"owner_lines"`
	Steps          []string `json:"steps"`
	LeftQRImage    string   `json:"left_qr_image"`
	LeftQRCaption  string   `json:"left_qr_caption"`
	RightQRImage   string   `json:"right_qr_image"`
	RightQRCaption string   `json:"right_qr_caption"`
}

// DefaultRedeemCardProfile 与站长设计稿上的内容一致。
func DefaultRedeemCardProfile() RedeemCardProfile {
	return RedeemCardProfile{
		OwnerLabel:     "主理人",
		OwnerName:      "悠一",
		OwnerLines:     []string{"去探索 · TOP100 探索者", "NEXIS · Cobuilder"},
		Steps:          []string{"登录进入 aaccx.pw/login", "进入兑换页面输入兑换码", "额度即时到账"},
		LeftQRCaption:  "微信群 · 天才程序员聚集地",
		RightQRCaption: "官网 · aaccx.pw/login",
	}
}

type RedeemCard struct {
	ID           int64
	Token        string
	RedeemCodeID int64
	Theme        string
	Content      RedeemCardContent
	CreatedBy    *int64
	ViewCount    int
	LastViewedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// 附带兑换码当前状态，管理员在列表里能直接看到这张卡对应的码有没有被用掉。
	Code       string
	CodeType   string
	CodeValue  float64
	CodeStatus string
}

// RedeemCardPublicView 是 /card/<token> 页面拿到的数据，不含任何内部 ID。
type RedeemCardPublicView struct {
	Theme      string            `json:"theme"`
	Code       string            `json:"code"`
	CodeStatus string            `json:"code_status"`
	Content    RedeemCardContent `json:"content"`
	Profile    RedeemCardProfile `json:"profile"`
}

type RedeemCardSaveInput struct {
	RedeemCodeID int64
	Theme        string
	Content      RedeemCardContent
	AdminID      int64
}

type RedeemCardService struct {
	entClient   *dbent.Client
	settingRepo SettingRepository
}

func NewRedeemCardService(entClient *dbent.Client, settingRepo SettingRepository) *RedeemCardService {
	return &RedeemCardService{entClient: entClient, settingRepo: settingRepo}
}

// List 按创建时间倒序列出卡片；search 按兑换码模糊匹配。
func (s *RedeemCardService) List(ctx context.Context, page, pageSize int, search string) ([]RedeemCard, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := s.entClient.RedeemCard.Query()
	if search = strings.TrimSpace(search); search != "" {
		codeIDs, err := s.entClient.RedeemCode.Query().
			Where(redeemcode.CodeContainsFold(search)).
			IDs(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("search redeem codes: %w", err)
		}
		if len(codeIDs) == 0 {
			return []RedeemCard{}, 0, nil
		}
		query = query.Where(redeemcard.RedeemCodeIDIn(codeIDs...))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count redeem cards: %w", err)
	}
	rows, err := query.
		Order(dbent.Desc(redeemcard.FieldCreatedAt), dbent.Desc(redeemcard.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list redeem cards: %w", err)
	}
	cards, err := s.withCodes(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return cards, int64(total), nil
}

// GetByCodeID 返回兑换码已有的卡片；没有时返回 nil。
func (s *RedeemCardService) GetByCodeID(ctx context.Context, redeemCodeID int64) (*RedeemCard, error) {
	row, err := s.entClient.RedeemCard.Query().Where(redeemcard.RedeemCodeID(redeemCodeID)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get redeem card by code: %w", err)
	}
	cards, err := s.withCodes(ctx, []*dbent.RedeemCard{row})
	if err != nil {
		return nil, err
	}
	return &cards[0], nil
}

// Save 为兑换码创建卡片，已有卡片则只更新卡面，链接（token）保持不变。
func (s *RedeemCardService) Save(ctx context.Context, input RedeemCardSaveInput) (*RedeemCard, error) {
	theme, err := normalizeRedeemCardTheme(input.Theme)
	if err != nil {
		return nil, err
	}
	content, err := normalizeRedeemCardContent(input.Content)
	if err != nil {
		return nil, err
	}
	exists, err := s.entClient.RedeemCode.Query().Where(redeemcode.ID(input.RedeemCodeID)).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("check redeem code: %w", err)
	}
	if !exists {
		return nil, ErrRedeemCardCodeNotFound
	}
	contentMap, err := redeemCardContentToMap(content)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	existing, err := s.entClient.RedeemCard.Query().Where(redeemcard.RedeemCodeID(input.RedeemCodeID)).Only(ctx)
	switch {
	case err == nil:
		if _, err := existing.Update().SetTheme(theme).SetContent(contentMap).SetUpdatedAt(now).Save(ctx); err != nil {
			return nil, fmt.Errorf("update redeem card: %w", err)
		}
	case dbent.IsNotFound(err):
		token, tokenErr := newRedeemCardToken()
		if tokenErr != nil {
			return nil, tokenErr
		}
		create := s.entClient.RedeemCard.Create().
			SetToken(token).
			SetRedeemCodeID(input.RedeemCodeID).
			SetTheme(theme).
			SetContent(contentMap).
			SetCreatedAt(now).
			SetUpdatedAt(now)
		if input.AdminID > 0 {
			create = create.SetCreatedBy(input.AdminID)
		}
		if _, err := create.Save(ctx); err != nil {
			if !dbent.IsConstraintError(err) {
				return nil, fmt.Errorf("create redeem card: %w", err)
			}
			// 两个管理员同时为同一个兑换码建卡：唯一索引拦下后一个，改成更新已建好的那张。
			// 查不到说明是别的约束（例如兑换码刚被删），原样报错，不能再重试。
			raced, getErr := s.entClient.RedeemCard.Query().Where(redeemcard.RedeemCodeID(input.RedeemCodeID)).Only(ctx)
			if getErr != nil {
				return nil, fmt.Errorf("create redeem card: %w", err)
			}
			if _, err := raced.Update().SetTheme(theme).SetContent(contentMap).SetUpdatedAt(now).Save(ctx); err != nil {
				return nil, fmt.Errorf("update redeem card: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("load redeem card: %w", err)
	}
	return s.GetByCodeID(ctx, input.RedeemCodeID)
}

// Delete 撤销卡片链接，兑换码本身不受影响。
func (s *RedeemCardService) Delete(ctx context.Context, id int64) error {
	err := s.entClient.RedeemCard.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return ErrRedeemCardNotFound
	}
	if err != nil {
		return fmt.Errorf("delete redeem card: %w", err)
	}
	return nil
}

// GetProfile 读取共用卡面信息；从未保存过时返回设计稿默认值。
func (s *RedeemCardService) GetProfile(ctx context.Context) (RedeemCardProfile, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyRedeemCardProfile)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && strings.TrimSpace(raw) == "") {
		return DefaultRedeemCardProfile(), nil
	}
	if err != nil {
		return RedeemCardProfile{}, fmt.Errorf("get redeem card profile: %w", err)
	}
	var profile RedeemCardProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return RedeemCardProfile{}, fmt.Errorf("decode redeem card profile: %w", err)
	}
	return profile, nil
}

func (s *RedeemCardService) UpdateProfile(ctx context.Context, profile RedeemCardProfile) (RedeemCardProfile, error) {
	normalized, err := normalizeRedeemCardProfile(profile)
	if err != nil {
		return RedeemCardProfile{}, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return RedeemCardProfile{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyRedeemCardProfile, string(payload)); err != nil {
		return RedeemCardProfile{}, fmt.Errorf("save redeem card profile: %w", err)
	}
	return normalized, nil
}

// GetPublic 供 /card/<token> 页面使用，并累加浏览次数。
func (s *RedeemCardService) GetPublic(ctx context.Context, token string) (*RedeemCardPublicView, error) {
	token = strings.ToLower(strings.TrimSpace(token))
	// 格式不对的 token 直接当不存在处理，不去查库。
	if !redeemCardTokenPattern.MatchString(token) {
		return nil, ErrRedeemCardNotFound
	}
	row, err := s.entClient.RedeemCard.Query().Where(redeemcard.Token(token)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, ErrRedeemCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get redeem card: %w", err)
	}
	code, err := s.entClient.RedeemCode.Get(ctx, row.RedeemCodeID)
	if dbent.IsNotFound(err) {
		return nil, ErrRedeemCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get redeem card code: %w", err)
	}
	profile, err := s.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	// 浏览数只是参考，失败不影响用户看卡。
	_ = s.entClient.RedeemCard.UpdateOneID(row.ID).AddViewCount(1).SetLastViewedAt(time.Now()).Exec(ctx)

	return &RedeemCardPublicView{
		Theme:      row.Theme,
		Code:       code.Code,
		CodeStatus: redeemCardCodeStatus(code, time.Now()),
		Content:    redeemCardContentFromMap(row.Content),
		Profile:    profile,
	}, nil
}

func (s *RedeemCardService) withCodes(ctx context.Context, rows []*dbent.RedeemCard) ([]RedeemCard, error) {
	codeIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		codeIDs = append(codeIDs, row.RedeemCodeID)
	}
	codes := make(map[int64]*dbent.RedeemCode, len(codeIDs))
	if len(codeIDs) > 0 {
		codeRows, err := s.entClient.RedeemCode.Query().Where(redeemcode.IDIn(codeIDs...)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("load redeem codes for cards: %w", err)
		}
		for _, code := range codeRows {
			codes[code.ID] = code
		}
	}
	now := time.Now()
	cards := make([]RedeemCard, 0, len(rows))
	for _, row := range rows {
		card := RedeemCard{
			ID:           row.ID,
			Token:        row.Token,
			RedeemCodeID: row.RedeemCodeID,
			Theme:        row.Theme,
			Content:      redeemCardContentFromMap(row.Content),
			CreatedBy:    row.CreatedBy,
			ViewCount:    row.ViewCount,
			LastViewedAt: row.LastViewedAt,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		if code := codes[row.RedeemCodeID]; code != nil {
			card.Code = code.Code
			card.CodeType = code.Type
			card.CodeValue = code.Value
			card.CodeStatus = redeemCardCodeStatus(code, now)
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// redeemCardCodeStatus 与 RedeemCode.IsExpiredAt 同口径：未使用但已过期的码显示为 expired。
func redeemCardCodeStatus(code *dbent.RedeemCode, now time.Time) string {
	if code.Status == StatusUnused && code.ExpiresAt != nil && !code.ExpiresAt.After(now) {
		return StatusExpired
	}
	return code.Status
}

func newRedeemCardToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate redeem card token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func normalizeRedeemCardTheme(theme string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "", RedeemCardThemeDark:
		return RedeemCardThemeDark, nil
	case RedeemCardThemeLight:
		return RedeemCardThemeLight, nil
	default:
		return "", infraerrors.BadRequest("REDEEM_CARD_INVALID_THEME", "theme must be dark or light")
	}
}

func normalizeRedeemCardContent(content RedeemCardContent) (RedeemCardContent, error) {
	out := RedeemCardContent{
		Amount:      strings.TrimSpace(content.Amount),
		Plan:        strings.TrimSpace(content.Plan),
		Tokens:      strings.TrimSpace(content.Tokens),
		ValidUntil:  strings.TrimSpace(content.ValidUntil),
		Serial:      strings.TrimSpace(content.Serial),
		HeatmapSeed: content.HeatmapSeed,
	}
	for field, value := range map[string]string{
		"amount": out.Amount, "plan": out.Plan, "tokens": out.Tokens, "valid_until": out.ValidUntil,
	} {
		if utf8.RuneCountInString(value) > redeemCardTextMaxRunes {
			return RedeemCardContent{}, redeemCardFieldTooLong(field, redeemCardTextMaxRunes)
		}
	}
	if utf8.RuneCountInString(out.Serial) > redeemCardSerialMaxRunes {
		return RedeemCardContent{}, redeemCardFieldTooLong("serial", redeemCardSerialMaxRunes)
	}
	if out.HeatmapSeed <= 0 {
		out.HeatmapSeed = redeemCardDefaultHeatSeed
	}
	if out.HeatmapSeed > redeemCardMaxHeatSeed {
		return RedeemCardContent{}, infraerrors.BadRequest("REDEEM_CARD_INVALID_FIELD", fmt.Sprintf("heatmap_seed must be between 1 and %d", redeemCardMaxHeatSeed))
	}
	return out, nil
}

func normalizeRedeemCardProfile(profile RedeemCardProfile) (RedeemCardProfile, error) {
	out := RedeemCardProfile{
		OwnerLabel:     strings.TrimSpace(profile.OwnerLabel),
		OwnerName:      strings.TrimSpace(profile.OwnerName),
		LeftQRImage:    strings.TrimSpace(profile.LeftQRImage),
		LeftQRCaption:  strings.TrimSpace(profile.LeftQRCaption),
		RightQRImage:   strings.TrimSpace(profile.RightQRImage),
		RightQRCaption: strings.TrimSpace(profile.RightQRCaption),
	}
	for field, value := range map[string]string{
		"owner_label": out.OwnerLabel, "owner_name": out.OwnerName,
		"left_qr_caption": out.LeftQRCaption, "right_qr_caption": out.RightQRCaption,
	} {
		if utf8.RuneCountInString(value) > redeemCardTextMaxRunes {
			return RedeemCardProfile{}, redeemCardFieldTooLong(field, redeemCardTextMaxRunes)
		}
	}
	var err error
	if out.OwnerLines, err = normalizeRedeemCardLines("owner_lines", profile.OwnerLines, redeemCardMaxOwnerLines); err != nil {
		return RedeemCardProfile{}, err
	}
	if out.Steps, err = normalizeRedeemCardLines("steps", profile.Steps, redeemCardMaxSteps); err != nil {
		return RedeemCardProfile{}, err
	}
	for field, image := range map[string]string{"left_qr_image": out.LeftQRImage, "right_qr_image": out.RightQRImage} {
		if err := validateRedeemCardImage(field, image); err != nil {
			return RedeemCardProfile{}, err
		}
	}
	return out, nil
}

func normalizeRedeemCardLines(field string, lines []string, maxLines int) ([]string, error) {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > redeemCardTextMaxRunes {
			return nil, redeemCardFieldTooLong(field, redeemCardTextMaxRunes)
		}
		out = append(out, line)
	}
	if len(out) > maxLines {
		return nil, infraerrors.BadRequest("REDEEM_CARD_INVALID_FIELD", fmt.Sprintf("%s supports at most %d lines", field, maxLines))
	}
	return out, nil
}

// validateRedeemCardImage 只接受 png / jpeg / webp 的 data URL，空字符串表示用站点默认图。
func validateRedeemCardImage(field, image string) error {
	if image == "" {
		return nil
	}
	match := redeemCardImagePattern.FindStringSubmatch(image)
	if match == nil {
		return infraerrors.BadRequest("REDEEM_CARD_INVALID_IMAGE", field+" must be a png, jpeg or webp data URL")
	}
	decoded, err := base64.StdEncoding.DecodeString(match[2])
	if err != nil {
		return infraerrors.BadRequest("REDEEM_CARD_INVALID_IMAGE", field+" is not valid base64")
	}
	if len(decoded) > redeemCardImageMaxBytes {
		return infraerrors.BadRequest("REDEEM_CARD_INVALID_IMAGE", fmt.Sprintf("%s must be at most %d KB", field, redeemCardImageMaxBytes/1024))
	}
	return nil
}

func redeemCardFieldTooLong(field string, max int) error {
	return infraerrors.BadRequest("REDEEM_CARD_INVALID_FIELD", fmt.Sprintf("%s must be at most %d characters", field, max))
}

func redeemCardContentToMap(content RedeemCardContent) (map[string]any, error) {
	payload, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func redeemCardContentFromMap(raw map[string]any) RedeemCardContent {
	var content RedeemCardContent
	payload, err := json.Marshal(raw)
	if err == nil {
		_ = json.Unmarshal(payload, &content)
	}
	if content.HeatmapSeed <= 0 {
		content.HeatmapSeed = redeemCardDefaultHeatSeed
	}
	return content
}
