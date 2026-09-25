-- 兑换卡：管理员为兑换码制作的分享卡片，用户通过 /card/<token> 打开看到 3D 兑换卡。
-- token 与兑换码无关，撤销链接只删这一行；兑换码被删时卡片随之删除。
-- 列名/可空性必须与 backend/ent/schema/redeem_card.go 保持一致（ent 不做自动迁移）。
CREATE TABLE IF NOT EXISTS redeem_cards (
    id             BIGSERIAL PRIMARY KEY,
    token          VARCHAR(64)  NOT NULL,
    redeem_code_id BIGINT       NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    theme          VARCHAR(16)  NOT NULL DEFAULT 'dark',
    content        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_by     BIGINT       NULL,
    view_count     INTEGER      NOT NULL DEFAULT 0,
    last_viewed_at TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS redeem_cards_token_key ON redeem_cards (token);
CREATE UNIQUE INDEX IF NOT EXISTS redeem_cards_redeem_code_id_key ON redeem_cards (redeem_code_id);
CREATE INDEX IF NOT EXISTS idx_redeem_cards_created_at ON redeem_cards (created_at);

COMMENT ON TABLE redeem_cards IS '兑换码分享卡片（3D 兑换卡），一个兑换码最多一张';
COMMENT ON COLUMN redeem_cards.content IS '卡面文字：amount/plan/tokens/valid_until/serial/heatmap_seed';
