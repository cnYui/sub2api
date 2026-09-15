-- 报销/开票信息申请：用户粘贴纯文本，经 LLM（DeepSeek）解析成 6 个结构化字段并补齐后才入库；
-- 管理员上传发票 PDF 即置为 completed。PDF 本体落在应用数据目录，这里只存相对路径与摘要。
-- 列名/可空性必须与 backend/ent/schema/reimbursement_request.go 保持一致（ent 不做自动迁移）。
CREATE TABLE IF NOT EXISTS reimbursement_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    raw_text TEXT NOT NULL DEFAULT '',
    company_name VARCHAR(255) NOT NULL,
    tax_id VARCHAR(64) NOT NULL,
    bank_account VARCHAR(64) NOT NULL,
    bank_name VARCHAR(255) NOT NULL,
    address VARCHAR(500) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    pdf_path VARCHAR(500) NOT NULL DEFAULT '',
    pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    pdf_size BIGINT NOT NULL DEFAULT 0,
    pdf_sha256 VARCHAR(64) NOT NULL DEFAULT '',
    pdf_uploaded_at TIMESTAMPTZ NULL,
    handled_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    completed_at TIMESTAMPTZ NULL,
    notified_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reimbursement_requests_status_check CHECK (status IN ('pending', 'completed')),
    CONSTRAINT reimbursement_requests_amount_check CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_reimbursement_requests_user_created
    ON reimbursement_requests (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reimbursement_requests_status_created
    ON reimbursement_requests (status, created_at);
