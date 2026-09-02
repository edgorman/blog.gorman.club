import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ReactionCount } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { ReactionBar } from './ReactionBar'

function renderBar(counts: ReactionCount[], canReact = true) {
  const onToggle = vi.fn()
  renderWithApp(<ReactionBar counts={counts} onToggle={onToggle} canReact={canReact} label="post" />)
  return onToggle
}

describe('ReactionBar', () => {
  // Signed in, all five are always there to click - not just the ones somebody has already
  // chosen - so reacting is one click rather than a click to open a picker and a second to choose.
  it('shows all five reactions to a signed-in reader, even unchosen ones', () => {
    renderBar([{ emoji: '👍', count: 3, reacted: false }])

    expect(screen.getByRole('button', { name: '👍 3' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '👎 0' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '❤️ 0' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '😄 0' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '🎉 0' })).toBeInTheDocument()
  })

  // A chip is a toggle, and being pressed is what says the reader is counted in it.
  it('marks the reactions the reader is in', () => {
    renderBar([
      { emoji: '👍', count: 3, reacted: true },
      { emoji: '🎉', count: 1, reacted: false },
    ])

    expect(screen.getByRole('button', { name: '👍 3' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: '🎉 1' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('toggles a reaction when its chip is clicked', async () => {
    const onToggle = renderBar([{ emoji: '👍', count: 3, reacted: false }])

    await userEvent.click(screen.getByRole('button', { name: '👍 3' }))

    expect(onToggle).toHaveBeenCalledWith('👍')
  })

  // The unchosen chips are just as clickable as the chosen ones - there is no separate picker.
  it('reacts with an emoji nobody has chosen yet', async () => {
    const onToggle = renderBar([])

    await userEvent.click(screen.getByRole('button', { name: '🎉 0' }))

    expect(onToggle).toHaveBeenCalledWith('🎉')
  })

  // Reading a bar never needed a credential; being counted in one does. An unchosen emoji is
  // hidden rather than shown greyed out, since there is nothing to look at and nothing to do.
  it('shows a signed-out reader only the chosen counts, with nothing to click', () => {
    renderBar([{ emoji: '👍', count: 3, reacted: false }], false)

    expect(screen.getByRole('button', { name: '👍 3' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '👎 0' })).not.toBeInTheDocument()
  })

  // Nothing to show and nothing to click is no bar at all, rather than a row of empty furniture.
  it('renders nothing for a signed-out reader when there are no reactions', () => {
    const { container } = renderWithApp(
      <ReactionBar counts={[]} onToggle={vi.fn()} canReact={false} label="post" />,
    )

    expect(container.querySelector('.reactions')).toBeNull()
  })
})
