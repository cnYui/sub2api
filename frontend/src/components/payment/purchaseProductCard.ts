export interface PurchaseProductCardRow {
  label: string
  value: string
}

export interface PurchaseProductCardModel {
  testId?: string
  eyebrowText: string
  title: string
  priceLabel: string
  priceText: string
  detailRows: PurchaseProductCardRow[]
  buttonText: string
  active?: boolean
}
