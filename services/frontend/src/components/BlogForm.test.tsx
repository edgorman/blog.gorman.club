import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BlogForm } from './BlogForm'

describe('BlogForm', () => {
  it('submits the entered fields', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<BlogForm onSubmit={onSubmit} />)

    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'Hello' } })
    fireEvent.change(screen.getByLabelText(/content/i), { target: { value: 'World' } })
    fireEvent.click(screen.getByRole('button', { name: /create/i }))

    await vi.waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit).toHaveBeenCalledWith({
      title: 'Hello',
      content: 'World',
      visibility: 'public',
      allowedUserIds: undefined,
    })
  })

  it('parses allowed user ids only when private', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<BlogForm onSubmit={onSubmit} />)

    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'T' } })
    fireEvent.change(screen.getByLabelText(/content/i), { target: { value: 'C' } })
    fireEvent.change(screen.getByLabelText(/visibility/i), { target: { value: 'private' } })
    fireEvent.change(screen.getByLabelText(/allowed user ids/i), {
      target: { value: 'u1, u2 ,, u3' },
    })
    fireEvent.click(screen.getByRole('button', { name: /create/i }))

    await vi.waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ visibility: 'private', allowedUserIds: ['u1', 'u2', 'u3'] }),
    )
  })

  it('shows an error message when submission fails', async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error('boom'))
    render(<BlogForm onSubmit={onSubmit} />)

    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'T' } })
    fireEvent.change(screen.getByLabelText(/content/i), { target: { value: 'C' } })
    fireEvent.click(screen.getByRole('button', { name: /create/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
  })
})
