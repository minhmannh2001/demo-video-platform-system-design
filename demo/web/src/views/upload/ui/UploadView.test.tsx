import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { UploadView } from './UploadView'

describe('UploadView', () => {
  it('renders form card and description', () => {
    render(
      <MemoryRouter>
        <UploadView />
      </MemoryRouter>,
    )
    expect(screen.getByText('Upload a video')).toBeInTheDocument()
    expect(screen.getByText(/once the API saves it/i)).toBeInTheDocument()
    expect(screen.getByTestId('upload-form')).toBeInTheDocument()
  })
})
