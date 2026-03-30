import { createContext, useContext, useState } from 'react'
import type { ReactNode } from 'react'
import type { User } from '@/types'

interface AuthContextValue {
  user:     User | null
  token:    string | null
  isSeller: boolean
  login:    (user: User, token: string) => void
  logout:   () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

function readStorage<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user,  setUser]  = useState<User | null>(() => readStorage<User>('auth_user'))
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('auth_token'))

  const login = (u: User, t: string) => {
    setUser(u)
    setToken(t)
    localStorage.setItem('auth_user',  JSON.stringify(u))
    localStorage.setItem('auth_token', t)
  }

  const logout = () => {
    setUser(null)
    setToken(null)
    localStorage.removeItem('auth_user')
    localStorage.removeItem('auth_token')
  }

  const isSeller = user?.role === 'seller'

  return (
    <AuthContext.Provider value={{ user, token, isSeller, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>')
  return ctx
}
