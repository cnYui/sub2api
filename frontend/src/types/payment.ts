/**
 * Payment System Type Definitions
 */

// ==================== Enums / Union Types ====================

export type OrderStatus =
  | 'PENDING'
  | 'PAID'
  | 'RECHARGING'
  | 'COMPLETED'
  | 'COMPENSATED'
  | 'EXPIRED'
  | 'CANCELLED'
  | 'FAILED'
  | 'REFUND_REQUESTED'
  | 'REFUNDING'
  | 'PARTIALLY_REFUNDED'
  | 'REFUNDED'
  | 'REFUND_FAILED'

export type PaymentType = 'alipay' | 'wxpay' | 'alipay_direct' | 'wxpay_direct' | 'stripe' | 'easypay' | 'airwallex' | 'balance'

export type OrderType = 'balance' | 'subscription' | 'traffic_pack'

// ==================== Configuration ====================

export interface PaymentConfig {
  payment_enabled: boolean
  min_amount: number
  max_amount: number
  daily_limit: number
  max_pending_orders: number
  order_timeout_minutes: number
  balance_disabled: boolean
  enabled_payment_types: PaymentType[]
  help_image_url: string
  help_text: string
  stripe_publishable_key: string
}

export interface MethodLimit {
  currency?: string
  daily_limit: number
  daily_used: number
  daily_remaining: number
  single_min: number
  single_max: number
  fee_rate: number
  available: boolean
}

/** Response from /payment/limits API */
export interface MethodLimitsResponse {
  methods: Record<string, MethodLimit>
  global_min: number  // widest min across all methods; 0 = no minimum
  global_max: number  // widest max across all methods; 0 = no maximum
}

/** Response from /payment/checkout-info API — single call for the payment page */
export interface CheckoutInfoResponse {
  methods: Record<string, MethodLimit>
  global_min: number
  global_max: number
  plans: SubscriptionPlan[]
  traffic_packs: TrafficPack[]
  traffic_credit_summary?: TrafficCreditSummary | null
  traffic_credits: TrafficCredit[]
  balance_disabled: boolean
  recharge_fee_rate: number
  help_text: string
  help_image_url: string
  stripe_publishable_key: string
  /** When true, Alipay payments on mobile always show the QR code instead of redirecting */
  alipay_force_qrcode?: boolean
}

export interface TrafficPack {
  id: number
  code: string
  name: string
  description: string
  price: number
  credit_usd: number
  validity_days: number
  platform: string
  for_sale: boolean
  sort_order: number
}

export interface TrafficCreditSummary {
  total_initial_usd: number
  total_remaining_usd: number
  next_expiring_usd: number
  next_expires_at?: string
}

export interface TrafficCredit {
  id: number
  order_id: number | null
  pack_id: number | null
  initial_usd: number
  remaining_usd: number
  reserved_usd: number
  available_usd: number
  credited_at: string
  expires_at: string
}

// ==================== Orders ====================

export interface PaymentOrder {
  id: number
  user_id: number
  amount: number
  pay_amount: number
  currency?: string
  fee_rate: number
  payment_type: string
  out_trade_no: string
  status: OrderStatus
  order_type: OrderType
  created_at: string
  expires_at: string
  paid_at?: string
  completed_at?: string
  refund_amount: number
  refund_reason?: string
  refund_requested_at?: string
  refund_requested_by?: number
  refund_request_reason?: string
  refund_retryable?: boolean
  plan_id?: number
  provider_instance_id?: string
  funding_mode?: 'gateway' | 'balance' | 'mixed' | string
  balance_amount?: number
  gateway_amount?: number
  payment_resolution_status?: 'PAID' | 'UNPAID' | 'UNKNOWN' | string
  payment_resolution_deadline?: string
  compensation_amount?: number
  compensated_at?: string
  refund_balance_amount?: number
  refund_gateway_amount?: number
  refund_balance_status?: string
  force_refund?: boolean
  subscription_snapshot?: Record<string, unknown>
  refund_basis?: Record<string, unknown>
}

export interface SubscriptionRefundQuote {
  eligible: boolean
  manual_review_required: boolean
  entitlement_period_id?: number
  purchase_base_amount: number
  non_refundable_fee: number
  period_total_quota_usd: number
  used_quota_usd: number
  usage_ratio: number
  time_ratio: number
  consumption_ratio: number
  estimated_refund_amount: number
  calculated_at: string
}

// ==================== Plans & Channels ====================

export interface SubscriptionPlan {
  id: number
  group_id: number
  group_platform?: string
  group_name?: string
  rate_multiplier?: number
  daily_limit_usd?: number | null
  weekly_limit_usd?: number | null
  monthly_limit_usd?: number | null
  period_total_quota_usd?: number | null
  quota_window_unit?: 'day' | 'week' | 'month' | 'none' | string
  quota_window_days?: number
  effective_validity_days?: number
  supported_model_scopes?: string[]
  name: string
  description: string
  price: number
  original_price?: number
  validity_days: number
  validity_unit: string
  /** Stored as JSON string in backend; API layer should parse before use */
  features: string[]
  for_sale: boolean
  sort_order: number
}

export interface PaymentChannel {
  id: number
  group_id?: number
  name: string
  platform: string
  rate_multiplier: number
  description: string
  models: string[]
  features: string[]
  enabled: boolean
}

// ==================== Providers ====================

export interface ProviderInstance {
  id: number
  provider_key: string
  name: string
  config: Record<string, string>
  supported_types: string[]
  enabled: boolean
  payment_mode: string
  refund_enabled: boolean
  allow_user_refund: boolean
  limits: string
  sort_order: number
}

// ==================== Request / Response ====================

export interface CreateOrderRequest {
  amount: number
  payment_type: string
  order_type: string
  plan_id?: number
  traffic_pack_id?: number
  return_url?: string
  payment_source?: string
  openid?: string
  wechat_resume_token?: string
  is_mobile?: boolean
  use_balance?: boolean
  expected_pay_amount?: string
  expected_balance_amount?: string
}

export interface BalancePayOrderRequest {
  order_type: 'subscription' | 'traffic_pack'
  plan_id?: number
  traffic_pack_id?: number
}

export type CreateOrderResultType = 'order_created' | 'oauth_required' | 'jsapi_ready'

export interface WechatOAuthInfo {
  authorize_url?: string
  appid?: string
  openid?: string
  scope?: string
  state?: string
  redirect_url?: string
}

export interface WechatJSAPIPayload {
  appId?: string
  timeStamp?: string
  nonceStr?: string
  package?: string
  signType?: string
  paySign?: string
}

export interface CreateOrderResult {
  order_id: number
  amount: number
  pay_url?: string
  qr_code?: string
  qr_image_url?: string
  client_secret?: string
  intent_id?: string
  currency?: string
  country_code?: string
  payment_env?: string
  pay_amount: number
  fee_rate: number
  expires_at: string
  result_type?: CreateOrderResultType
  payment_type?: string
  out_trade_no?: string
  payment_mode?: string
  resume_token?: string
  funding_mode?: 'gateway' | 'balance' | 'mixed' | string
  balance_amount?: number
  gateway_amount?: number
  payment_resolution_status?: 'PAID' | 'UNPAID' | 'UNKNOWN' | string
  payment_resolution_deadline?: string
  compensation_amount?: number
  compensated_at?: string
  oauth?: WechatOAuthInfo
  jsapi?: WechatJSAPIPayload
  jsapi_payload?: WechatJSAPIPayload
}

export interface DashboardStats {
  today_amount: number
  total_amount: number
  today_count: number
  total_count: number
  avg_amount: number
  daily_series: { date: string; amount: number; count: number }[]
  payment_methods: { type: string; amount: number; count: number }[]
  top_users: { user_id: number; email: string; amount: number }[]
}
