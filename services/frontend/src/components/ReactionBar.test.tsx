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
  it('draws a chip per emoji with its count', () => {
    renderBar([
      { emoji: '👍', count: 3, reacted: false },
      { emoji: '🎉', count: 1, reacted: false },
    ])

    expect(screen.getByRole('button', { name: '👍 3' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '🎉 1' })).toBeInTheDocument()
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

  it('reacts with an emoji chosen from the picker', async () => {
    const onToggle = renderBar([])

    await userEvent.click(screen.getByRole('button', { name: 'React to this post' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'React with 🎉' }))

    expect(onToggle).toHaveBeenCalledWith('🎉')
    // Chosen, so the picker closes rather than staying open over the page.
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('offers more than a handful of emoji to choose from', async () => {
    renderBar([])

    await userEvent.click(screen.getByRole('button', { name: 'React to this post' }))

    expect(screen.getAllByRole('menuitem').length).toBeGreaterThan(5)
  })

  // Reading a bar never needed a credential; being counted in one does.
  it('shows a signed-out reader the counts without anything to click', () => {
    renderBar([{ emoji: '👍', count: 3, reacted: false }], false)

    expect(screen.getByRole('button', { name: '👍 3' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'React to this post' })).not.toBeInTheDocument()
  })

  // Nothing to show and nothing to click is no bar at all, rather than a row of empty furniture.
  it('renders nothing for a signed-out reader when there are no reactions', () => {
    const { container } = renderWithApp(
      <ReactionBar counts={[]} onToggle={vi.fn()} canReact={false} label="post" />,
    )

    expect(container.querySelector('.reactions')).toBeNull()
  })
})
