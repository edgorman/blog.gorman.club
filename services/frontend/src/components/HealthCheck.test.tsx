import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { HealthCheck } from './HealthCheck'

describe('HealthCheck', () => {
  it('shows a placeholder when no backend is configured', () => {
    render(<HealthCheck />)

    expect(screen.getByText(/no backend deployed yet/i)).toBeInTheDocument()
  })
})
