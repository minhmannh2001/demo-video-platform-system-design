import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode }

export function Button({ children, type = 'button', ...rest }: Props) {
  return (
    <button type={type} className="btn" {...rest}>
      {children}
    </button>
  )
}
