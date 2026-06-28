export interface PurchaseProductCardRow {
  label: string
  value: string
}

export interface PurchaseProductCardModel {
  testId?: string
  title: string
  priceText: string
  detailRows: PurchaseProductCardRow[]
  buttonText: string
  active?: boolean
}

