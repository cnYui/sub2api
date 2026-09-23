package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func newRedeemCardTestService(t *testing.T) (*RedeemCardService, *dbent.Client, *notificationEmailMemorySettingRepo) {
	t.Helper()
	client := newPaymentConfigServiceTestClient(t)
	repo := newNotificationEmailMemorySettingRepo()
	return NewRedeemCardService(client, repo), client, repo
}

func createRedeemCardTestCode(t *testing.T, client *dbent.Client, code, status string, expiresAt *time.Time) *dbent.RedeemCode {
	t.Helper()
	create := client.RedeemCode.Create().SetCode(code).SetType(RedeemTypeBalance).SetValue(10).SetStatus(status)
	if expiresAt != nil {
		create = create.SetExpiresAt(*expiresAt)
	}
	row, err := create.Save(context.Background())
	require.NoError(t, err)
	return row
}

func TestRedeemCardSaveKeepsTokenAcrossEdits(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newRedeemCardTestService(t)
	code := createRedeemCardTestCode(t, client, "7F3K9QXA2M8DTC01", StatusUnused, nil)

	first, err := svc.Save(ctx, RedeemCardSaveInput{
		RedeemCodeID: code.ID,
		Content:      RedeemCardContent{Amount: " ¥100 ", Plan: "Pro 套餐 · 30 天", Tokens: "1000 万 Tokens", ValidUntil: "2026-12-31", Serial: "0001"},
		AdminID:      448,
	})
	require.NoError(t, err)
	require.Regexp(t, `^[0-9a-f]{32}$`, first.Token)
	require.Equal(t, RedeemCardThemeDark, first.Theme, "empty theme defaults to the dark card")
	require.Equal(t, "¥100", first.Content.Amount, "text is trimmed")
	require.Equal(t, redeemCardDefaultHeatSeed, first.Content.HeatmapSeed)
	require.Equal(t, "7F3K9QXA2M8DTC01", first.Code)
	require.Equal(t, StatusUnused, first.CodeStatus)
	require.NotNil(t, first.CreatedBy)

	second, err := svc.Save(ctx, RedeemCardSaveInput{
		RedeemCodeID: code.ID,
		Theme:        RedeemCardThemeLight,
		Content:      RedeemCardContent{Amount: "¥200", HeatmapSeed: 42},
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "one card per redeem code")
	require.Equal(t, first.Token, second.Token, "editing a card must not change the link already sent to the user")
	require.Equal(t, RedeemCardThemeLight, second.Theme)
	require.Equal(t, "¥200", second.Content.Amount)
	require.Equal(t, 42, second.Content.HeatmapSeed)

	count, err := client.RedeemCard.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestRedeemCardSaveValidatesInput(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newRedeemCardTestService(t)
	code := createRedeemCardTestCode(t, client, "VALIDATE0001", StatusUnused, nil)

	_, err := svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID, Theme: "neon"})
	require.ErrorContains(t, err, "theme")

	_, err = svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID, Content: RedeemCardContent{Plan: strings.Repeat("长", redeemCardTextMaxRunes+1)}})
	require.ErrorContains(t, err, "plan")

	_, err = svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID, Content: RedeemCardContent{HeatmapSeed: redeemCardMaxHeatSeed + 1}})
	require.ErrorContains(t, err, "heatmap_seed")

	_, err = svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID + 999})
	require.ErrorIs(t, err, ErrRedeemCardCodeNotFound)
}

func TestRedeemCardListSearchesByCodeAndReportsStatus(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newRedeemCardTestService(t)
	past := time.Now().Add(-time.Hour)
	unused := createRedeemCardTestCode(t, client, "ALPHA-UNUSED", StatusUnused, nil)
	used := createRedeemCardTestCode(t, client, "BETA-USED", StatusUsed, nil)
	expired := createRedeemCardTestCode(t, client, "ALPHA-EXPIRED", StatusUnused, &past)
	for _, code := range []*dbent.RedeemCode{unused, used, expired} {
		_, err := svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID})
		require.NoError(t, err)
	}

	all, total, err := svc.List(ctx, 1, 20, "")
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	statuses := map[string]string{}
	for _, card := range all {
		statuses[card.Code] = card.CodeStatus
	}
	require.Equal(t, StatusUnused, statuses["ALPHA-UNUSED"])
	require.Equal(t, StatusUsed, statuses["BETA-USED"])
	require.Equal(t, StatusExpired, statuses["ALPHA-EXPIRED"], "an unused code past expires_at shows as expired")

	alpha, total, err := svc.List(ctx, 1, 20, "alpha")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, alpha, 2)

	none, total, err := svc.List(ctx, 1, 20, "no-such-code")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, none)
}

func TestRedeemCardPublicViewByToken(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newRedeemCardTestService(t)
	code := createRedeemCardTestCode(t, client, "PUBLIC-CODE-01", StatusUnused, nil)
	card, err := svc.Save(ctx, RedeemCardSaveInput{
		RedeemCodeID: code.ID,
		Theme:        RedeemCardThemeLight,
		Content:      RedeemCardContent{Amount: "¥100", Serial: "0007"},
	})
	require.NoError(t, err)

	view, err := svc.GetPublic(ctx, strings.ToUpper(card.Token))
	require.NoError(t, err)
	require.Equal(t, "PUBLIC-CODE-01", view.Code)
	require.Equal(t, RedeemCardThemeLight, view.Theme)
	require.Equal(t, StatusUnused, view.CodeStatus)
	require.Equal(t, "0007", view.Content.Serial)
	require.Equal(t, DefaultRedeemCardProfile(), view.Profile, "profile falls back to the design defaults")

	_, err = svc.GetPublic(ctx, card.Token)
	require.NoError(t, err)
	stored, err := client.RedeemCard.Get(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, 2, stored.ViewCount)
	require.NotNil(t, stored.LastViewedAt)
	require.Equal(t, card.UpdatedAt.Unix(), stored.UpdatedAt.Unix(), "counting views must not look like an edit")

	for _, bad := range []string{"", "not-a-token", strings.Repeat("0", 32), card.Token + "0"} {
		_, err := svc.GetPublic(ctx, bad)
		require.ErrorIs(t, err, ErrRedeemCardNotFound, bad)
	}

	_, err = client.RedeemCode.UpdateOneID(code.ID).SetStatus(StatusUsed).Save(ctx)
	require.NoError(t, err)
	view, err = svc.GetPublic(ctx, card.Token)
	require.NoError(t, err)
	require.Equal(t, StatusUsed, view.CodeStatus)
}

func TestRedeemCardDeleteRevokesLinkButKeepsCode(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newRedeemCardTestService(t)
	code := createRedeemCardTestCode(t, client, "REVOKE-ME-0001", StatusUnused, nil)
	card, err := svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, card.ID))
	_, err = svc.GetPublic(ctx, card.Token)
	require.ErrorIs(t, err, ErrRedeemCardNotFound)
	exists, err := client.RedeemCode.Query().Where().Exist(ctx)
	require.NoError(t, err)
	require.True(t, exists, "revoking the card must not delete the redeem code")
	require.ErrorIs(t, svc.Delete(ctx, card.ID), ErrRedeemCardNotFound)

	// 重新做卡会得到新链接，旧链接保持失效。
	again, err := svc.Save(ctx, RedeemCardSaveInput{RedeemCodeID: code.ID})
	require.NoError(t, err)
	require.NotEqual(t, card.Token, again.Token)
}

func TestRedeemCardProfileValidation(t *testing.T) {
	ctx := context.Background()
	svc, _, repo := newRedeemCardTestService(t)

	profile, err := svc.GetProfile(ctx)
	require.NoError(t, err)
	require.Equal(t, "悠一", profile.OwnerName)

	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG fake"))
	saved, err := svc.UpdateProfile(ctx, RedeemCardProfile{
		OwnerLabel:  " 主理人 ",
		OwnerName:   "悠一",
		OwnerLines:  []string{"去探索 · TOP100 探索者", "   ", "NEXIS · Cobuilder"},
		Steps:       []string{"登录", "兑换", "到账"},
		LeftQRImage: png,
	})
	require.NoError(t, err)
	require.Equal(t, "主理人", saved.OwnerLabel)
	require.Equal(t, []string{"去探索 · TOP100 探索者", "NEXIS · Cobuilder"}, saved.OwnerLines, "blank lines are dropped")
	raw, err := repo.GetValue(ctx, SettingKeyRedeemCardProfile)
	require.NoError(t, err)
	require.Contains(t, raw, "data:image/png;base64,")

	reloaded, err := svc.GetProfile(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, reloaded)

	_, err = svc.UpdateProfile(ctx, RedeemCardProfile{LeftQRImage: "https://example.com/qr.png"})
	require.ErrorContains(t, err, "left_qr_image")
	_, err = svc.UpdateProfile(ctx, RedeemCardProfile{RightQRImage: "data:image/svg+xml;base64,PHN2Zz4="})
	require.ErrorContains(t, err, "right_qr_image", "svg can carry script and is rejected")
	tooBig := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, redeemCardImageMaxBytes+1))
	_, err = svc.UpdateProfile(ctx, RedeemCardProfile{LeftQRImage: tooBig})
	require.ErrorContains(t, err, "KB")
	_, err = svc.UpdateProfile(ctx, RedeemCardProfile{Steps: []string{"1", "2", "3", "4", "5", "6"}})
	require.ErrorContains(t, err, "steps")
}
