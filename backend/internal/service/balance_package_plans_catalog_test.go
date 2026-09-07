package service

import "testing"

// TestDefaultBalancePackagePlansCatalog 锁定余额套餐目录，重点保护新增的 ¥349~¥699 八档：
// 每周到账金额直接影响平台收入，误改一位数字就是漏钱，因此用精确期望值钉死，不用浮点公式重算
// （2.6 在 float64 下不可精确表示，price×2.6+3.6 会引入尾差）。同时校验 code 唯一、sortOrder 严格递增。
func TestDefaultBalancePackagePlansCatalog(t *testing.T) {
	want := []defaultBalancePackagePlan{
		{code: "balance-29", name: "余额套餐 ¥29", priceCNY: 29, weeklyCreditUSD: 76, sortOrder: 10},
		{code: "balance-39", name: "余额套餐 ¥39", priceCNY: 39, weeklyCreditUSD: 102, sortOrder: 20},
		{code: "balance-49", name: "余额套餐 ¥49", priceCNY: 49, weeklyCreditUSD: 128, sortOrder: 30},
		{code: "balance-59", name: "余额套餐 ¥59", priceCNY: 59, weeklyCreditUSD: 154, sortOrder: 40},
		{code: "balance-79", name: "余额套餐 ¥79", priceCNY: 79, weeklyCreditUSD: 206, sortOrder: 50},
		{code: "balance-99", name: "余额套餐 ¥99", priceCNY: 99, weeklyCreditUSD: 258, sortOrder: 60},
		{code: "balance-149", name: "余额套餐 ¥149", priceCNY: 149, weeklyCreditUSD: 389, sortOrder: 70},
		{code: "balance-199", name: "余额套餐 ¥199", priceCNY: 199, weeklyCreditUSD: 520, sortOrder: 80},
		{code: "balance-249", name: "余额套餐 ¥249", priceCNY: 249, weeklyCreditUSD: 651, sortOrder: 90},
		{code: "balance-299", name: "余额套餐 ¥299", priceCNY: 299, weeklyCreditUSD: 781, sortOrder: 100},
		{code: "balance-349", name: "余额套餐 ¥349", priceCNY: 349, weeklyCreditUSD: 911, sortOrder: 110},
		{code: "balance-399", name: "余额套餐 ¥399", priceCNY: 399, weeklyCreditUSD: 1041, sortOrder: 120},
		{code: "balance-449", name: "余额套餐 ¥449", priceCNY: 449, weeklyCreditUSD: 1171, sortOrder: 130},
		{code: "balance-499", name: "余额套餐 ¥499", priceCNY: 499, weeklyCreditUSD: 1301, sortOrder: 140},
		{code: "balance-549", name: "余额套餐 ¥549", priceCNY: 549, weeklyCreditUSD: 1431, sortOrder: 150},
		{code: "balance-599", name: "余额套餐 ¥599", priceCNY: 599, weeklyCreditUSD: 1561, sortOrder: 160},
		{code: "balance-649", name: "余额套餐 ¥649", priceCNY: 649, weeklyCreditUSD: 1691, sortOrder: 170},
		{code: "balance-699", name: "余额套餐 ¥699", priceCNY: 699, weeklyCreditUSD: 1821, sortOrder: 180},
	}

	if len(defaultBalancePackagePlans) != len(want) {
		t.Fatalf("plan count = %d, want %d", len(defaultBalancePackagePlans), len(want))
	}
	for i, w := range want {
		got := defaultBalancePackagePlans[i]
		if got != w {
			t.Errorf("plan[%d] = %+v, want %+v", i, got, w)
		}
	}

	seen := make(map[string]bool, len(defaultBalancePackagePlans))
	lastSort := -1
	for _, p := range defaultBalancePackagePlans {
		if seen[p.code] {
			t.Errorf("duplicate plan code %q", p.code)
		}
		seen[p.code] = true
		if p.sortOrder <= lastSort {
			t.Errorf("sortOrder not strictly increasing at %q: %d after %d", p.code, p.sortOrder, lastSort)
		}
		lastSort = p.sortOrder
		if p.priceCNY <= 0 || p.weeklyCreditUSD <= 0 {
			t.Errorf("plan %q has non-positive price/credit: %+v", p.code, p)
		}
	}
}
