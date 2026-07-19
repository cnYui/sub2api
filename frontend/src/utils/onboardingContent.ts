import type { DriveStep, Popover } from 'driver.js'

const EMOJI_PATTERN = /[\p{Extended_Pictographic}\uFE0F\u200D]/gu

const stripEmoji = (content: string): string => content
  .replace(EMOJI_PATTERN, '')
  .replace(/(^|>)\s+/g, '$1')
  .replace(/[ \t]{2,}/g, ' ')
  .trim()

const normalizePopoverText = (popover: Popover): Popover => ({
  ...popover,
  title: popover.title ? stripEmoji(popover.title) : popover.title,
  description: popover.description ? stripEmoji(popover.description) : popover.description,
  nextBtnText: popover.nextBtnText ? stripEmoji(popover.nextBtnText) : popover.nextBtnText,
  prevBtnText: popover.prevBtnText ? stripEmoji(popover.prevBtnText) : popover.prevBtnText,
  doneBtnText: popover.doneBtnText ? stripEmoji(popover.doneBtnText) : popover.doneBtnText,
})

export const normalizeOnboardingSteps = (steps: DriveStep[]): DriveStep[] => steps.map((step) => ({
  ...step,
  popover: step.popover ? normalizePopoverText(step.popover) : step.popover,
}))
