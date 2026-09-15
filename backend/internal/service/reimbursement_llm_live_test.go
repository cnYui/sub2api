//go:build unit

package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestReimbursementParser_Live 用真实 DeepSeek 跑契约第 7 节案例。
// 只在设置了 REIMBURSEMENT_LIVE_TEST_API_KEY 时执行，CI 上默认跳过；key 只从环境变量读，
// 绝不写进仓库。
func TestReimbursementParser_Live(t *testing.T) {
	apiKey := os.Getenv("REIMBURSEMENT_LIVE_TEST_API_KEY")
	if apiKey == "" {
		t.Skip("REIMBURSEMENT_LIVE_TEST_API_KEY not set; skipping live DeepSeek test")
	}
	cfg := &config.Config{}
	cfg.Reimbursement = config.ReimbursementConfig{
		LLMAPIKey:    apiKey,
		LLMBaseURL:   "https://api.deepseek.com",
		LLMModel:     "deepseek-flash",
		LLMTimeoutMS: 60000,
	}
	parser := NewReimbursementParser(newStubSettingRepo(), cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("case1", func(t *testing.T) {
		result, err := parser.Parse(ctx, reimbursementCase1Text, nil)
		require.NoError(t, err)
		require.True(t, result.Complete, "missing=%v notes=%v", result.Missing, result.Notes)
		require.Equal(t, "福州斯摩尔贸易有限公司", *result.Fields.CompanyName)
		require.Equal(t, "91350102068793190Q", *result.Fields.TaxID)
		require.Equal(t, "406565458594", *result.Fields.BankAccount)
		require.Equal(t, "中国银行福州东区支行", *result.Fields.BankName)
		require.Equal(t, "福州市鼓楼区鼓东街道井大路205号七星井新村B17座中侧3层354", *result.Fields.Address)
		require.Equal(t, float64(45), *result.Fields.Amount)
	})

	var case2 *ReimbursementParseResult
	t.Run("case2 missing address", func(t *testing.T) {
		result, err := parser.Parse(ctx, reimbursementCase2Text, nil)
		require.NoError(t, err)
		require.False(t, result.Complete)
		require.Equal(t, []string{"address"}, result.Missing)
		require.Equal(t, "上海熠视智能科技有限公司", *result.Fields.CompanyName)
		require.Equal(t, "91310115MAKFGD5NXY", *result.Fields.TaxID)
		require.Equal(t, "121995771710001", *result.Fields.BankAccount)
		require.Equal(t, "招商银行股份有限公司上海张杨支行", *result.Fields.BankName)
		require.Nil(t, result.Fields.Address)
		require.Equal(t, 414.1, *result.Fields.Amount)
		case2 = result
	})

	t.Run("case2 supplement", func(t *testing.T) {
		if case2 == nil {
			t.Skip("case2 failed")
		}
		result, err := parser.Parse(ctx, "地址是上海市浦东新区张杨路500号华润时代广场12楼", &case2.Fields)
		require.NoError(t, err)
		require.True(t, result.Complete, "missing=%v notes=%v", result.Missing, result.Notes)
		require.Equal(t, "上海市浦东新区张杨路500号华润时代广场12楼", *result.Fields.Address)
		require.Equal(t, *case2.Fields.CompanyName, *result.Fields.CompanyName)
		require.Equal(t, *case2.Fields.TaxID, *result.Fields.TaxID)
		require.Equal(t, *case2.Fields.BankAccount, *result.Fields.BankAccount)
		require.Equal(t, *case2.Fields.BankName, *result.Fields.BankName)
		require.Equal(t, *case2.Fields.Amount, *result.Fields.Amount)
	})

	t.Run("case3", func(t *testing.T) {
		result, err := parser.Parse(ctx, reimbursementCase3Text, nil)
		require.NoError(t, err)
		require.True(t, result.Complete, "missing=%v notes=%v", result.Missing, result.Notes)
		require.Equal(t, "北京和讯在线信息咨询服务有限公司", *result.Fields.CompanyName)
		require.Equal(t, "91110105723558454P", *result.Fields.TaxID)
		require.Equal(t, "0200080709024530517", *result.Fields.BankAccount)
		require.Equal(t, "中国工商银行股份有限公司北京东城支行", *result.Fields.BankName)
		require.Equal(t, "北京市朝阳区朝外大街22号10层A1-3", *result.Fields.Address)
		require.Equal(t, float64(49), *result.Fields.Amount)
	})

	t.Run("irrelevant text", func(t *testing.T) {
		result, err := parser.Parse(ctx, "你好，请问今天天气怎么样？", nil)
		require.NoError(t, err)
		require.False(t, result.Complete)
		require.Equal(t, ReimbursementFieldKeys, result.Missing)
		require.Equal(t, ReimbursementFields{}, result.Fields)
		require.NotNil(t, result.Notes)
		require.Contains(t, *result.Notes, "未识别到开票信息")
	})
}
