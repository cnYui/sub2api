import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
}))
const routerPush = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({ push: routerPush }),
  }
})

vi.mock('@/components/payment/PaymentStatusPanel.vue', () => ({
  default: {
    name: 'PaymentStatusPanel',
    props: ['orderId', 'qrCode', 'qrImageUrl', 'expiresAt', 'paymentType', 'payUrl', 'orderType', 'currency'],
    emits: ['done'],
    template: '<div data-testid="payment-status-panel" />',
  },
}))

import PaymentQRCodeView from '../PaymentQRCodeView.vue'

describe('PaymentQRCodeView', () => {
  it('passes legacy QR query parameters to the shared payment status panel', () => {
    routeState.query = {
      order_id: '42',
      qr: 'https://example.com/qr',
      qr_image_url: 'https://zpayz.cn/qrcode/123.jpg',
      pay_url: 'https://example.com/pay',
      expires_at: '2099-01-01T12:30:00Z',
      payment_type: 'alipay',
      order_type: 'traffic_pack',
      currency: 'CNY',
    }

    const wrapper = mount(PaymentQRCodeView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
        },
      },
    })

    const panel = wrapper.findComponent({ name: 'PaymentStatusPanel' })
    expect(panel.props()).toMatchObject({
      orderId: 42,
      qrCode: 'https://example.com/qr',
      qrImageUrl: 'https://zpayz.cn/qrcode/123.jpg',
      payUrl: 'https://example.com/pay',
      expiresAt: '2099-01-01T12:30:00Z',
      paymentType: 'alipay',
      orderType: 'traffic_pack',
      currency: 'CNY',
    })
  })

  it('returns to the purchase page after the shared panel finishes', async () => {
    routeState.query = {}
    routerPush.mockReset()

    const wrapper = mount(PaymentQRCodeView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
        },
      },
    })

    await wrapper.findComponent({ name: 'PaymentStatusPanel' }).vm.$emit('done')
    expect(routerPush).toHaveBeenCalledWith('/purchase')
  })
})
