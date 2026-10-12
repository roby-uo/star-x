/** Human-facing prices use CNY; existing wallet and quota APIs retain USD ledger units. */
export const USD_TO_CNY_RATE = 6.2

export type MoneyValue = number | string | null | undefined

function numericMoney(value: MoneyValue): number {
  const amount = typeof value === 'string' ? Number(value.replace(/,/g, '')) : Number(value ?? 0)
  return Number.isFinite(amount) ? amount : 0
}

export function ledgerToCny(amount: MoneyValue): number {
  return numericMoney(amount) * USD_TO_CNY_RATE
}

export function cnyToLedger(amount: MoneyValue): number {
  // Never round to cents: a one-cent CNY credit is smaller than a USD cent.
  return numericMoney(amount) / USD_TO_CNY_RATE
}

export function formatCny(amount: MoneyValue, fractionDigits?: number): string {
  const value = numericMoney(amount)
  let digits = fractionDigits ?? (value !== 0 && Math.abs(value) < 0.01 ? 6 : 2)
  if (value !== 0 && Math.abs(value) < 0.5 * 10 ** -digits) {
    digits = Math.min(12, Math.max(digits, Math.ceil(-Math.log10(Math.abs(value))) + 2))
  }
  return `¥${value.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: digits })}`
}

export function formatCnyFromLedger(amount: MoneyValue, fractionDigits?: number): string {
  return formatCny(ledgerToCny(amount), fractionDigits)
}

/** Use CNY for native cash payments and USD for historical ledger amounts. */
export function formatMoney(amount: MoneyValue, sourceCurrency = 'USD', fractionDigits?: number): string {
  const currency = sourceCurrency.trim().toUpperCase()
  if (currency === 'CNY' || currency === 'RMB') return formatCny(amount, fractionDigits)
  if (currency === 'USD' || currency === '') return formatCnyFromLedger(amount, fractionDigits)
  // Never invent an exchange rate for an unsupported payment currency.
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(numericMoney(amount))
}

export function ledgerFixed(amount: MoneyValue, fractionDigits = 2): string {
  return ledgerToCny(amount).toFixed(fractionDigits)
}
