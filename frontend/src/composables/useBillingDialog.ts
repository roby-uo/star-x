import { nextTick, onMounted, onUnmounted, type Ref } from 'vue'

/** Keep keyboard navigation inside a billing editor and restore its opener. */
export function useBillingDialog(element: Ref<HTMLElement | null>, close: () => void, locked: () => boolean) {
  let previous: HTMLElement | null = null
  let previousOverflow = ''
  const onKey = (event: KeyboardEvent) => {
    if (!element.value) return
    if (event.key === 'Escape') {
      event.preventDefault()
      if (!locked()) close()
      return
    }
    if (event.key !== 'Tab') return
    const focusable = [...element.value.querySelectorAll<HTMLElement>('button, input, select, textarea, summary, a[href], [tabindex="0"]')].filter(item => !item.matches(':disabled') && (item.tagName === 'SUMMARY' || item.closest('details:not([open])') == null) && !item.hasAttribute('hidden'))
    const first = focusable[0], last = focusable[focusable.length - 1]
    if (!first || !last) { event.preventDefault(); return }
    if (event.shiftKey && (document.activeElement === first || !element.value.contains(document.activeElement))) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && (document.activeElement === last || !element.value.contains(document.activeElement))) { event.preventDefault(); first.focus() }
  }
  onMounted(async () => {
    previous = document.activeElement as HTMLElement | null
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', onKey)
    await nextTick()
    element.value?.querySelector<HTMLElement>('button, input, select, textarea')?.focus()
  })
  onUnmounted(() => { document.removeEventListener('keydown', onKey); document.body.style.overflow = previousOverflow; previous?.focus() })
}
