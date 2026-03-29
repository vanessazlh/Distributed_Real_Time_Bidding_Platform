import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api, decodeToken } from '@/lib/api'
import { Card, Button, FormField, TextInput, StatusBanner } from '@/components/ui'
import { PageContainer } from '@/components/layout'

interface AuthPageProps {
  type: 'login' | 'register'
}

export default function AuthPage({ type }: AuthPageProps) {
  const { login }  = useAuth()
  const navigate   = useNavigate()
  const isLogin    = type === 'login'

  const [username, setUsername] = useState('')
  const [email,    setEmail]    = useState('')
  const [password, setPassword] = useState('')
  const [loading,  setLoading]  = useState(false)
  const [error,    setError]    = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      if (isLogin) {
        const { token } = await api.auth.login(email, password)
        const payload   = decodeToken(token)
        login(
          {
            user_id:  payload?.user_id  ?? '',
            username: payload?.username ?? email.split('@')[0],
            email,
          },
          token,
        )
        navigate('/')
      } else {
        await api.auth.register(username, email, password)
        // Registration done — send to login to obtain a token
        navigate('/login')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer>
      <div className="max-w-md mx-auto mt-8">
        <Card padding="p-8">
          <h1 className="font-display text-4xl text-brand text-center mb-2">SurpriseAuction</h1>
          <p className="text-center text-text-secondary text-sm mb-8">
            {isLogin ? 'Welcome back' : 'Create your account'}
          </p>

          {error && (
            <div className="mb-4">
              <StatusBanner type="error" message={error} />
            </div>
          )}

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {!isLogin && (
              <FormField label="Username">
                <TextInput
                  type="text"
                  required
                  placeholder="your_username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </FormField>
            )}
            <FormField label="Email">
              <TextInput
                type="email"
                required
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </FormField>
            <FormField label="Password">
              <TextInput
                type="password"
                required
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </FormField>

            <Button variant="primary" size="lg" type="submit" fullWidth disabled={loading} className="mt-2">
              {loading ? 'Please wait…' : isLogin ? 'Sign In' : 'Create Account'}
            </Button>
          </form>

          <p className="text-center text-sm text-text-secondary mt-6">
            {isLogin ? "Don't have an account? " : 'Already have an account? '}
            <Link
              to={isLogin ? '/register' : '/login'}
              className="text-brand font-medium hover:underline"
            >
              {isLogin ? 'Register' : 'Sign In'}
            </Link>
          </p>
        </Card>
      </div>
    </PageContainer>
  )
}
