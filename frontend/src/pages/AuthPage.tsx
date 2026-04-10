import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api, ApiError, decodeToken } from '@/lib/api'
import { Card, Button, FormField, TextInput, StatusBanner } from '@/components/ui'
import { PageContainer } from '@/components/layout'

interface AuthPageProps {
  type: 'login' | 'register'
}

export default function AuthPage({ type }: AuthPageProps) {
  const { login }  = useAuth()
  const navigate   = useNavigate()
  const isLogin    = type === 'login'

  const [role,     setRole]     = useState<'buyer' | 'seller'>('buyer')
  const [username, setUsername] = useState('')
  const [email,    setEmail]    = useState('')
  const [password, setPassword] = useState('')
  const [loading,  setLoading]  = useState(false)
  const [error,    setError]    = useState<string | null>(null)
  const [upgradePrompt, setUpgradePrompt] = useState<{ existingUsername: string } | null>(null)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      if (isLogin) {
        const { token } = await api.auth.login(email, password)
        const payload   = decodeToken(token)
        const jwtRole   = payload?.role ?? 'buyer'

        // If user picks Seller but the account has never been upgraded, reject.
        if (role === 'seller' && jwtRole !== 'seller') {
          setError('This account is not registered as a seller. Please sign up as a seller first.')
          setLoading(false)
          return
        }
        // Any account can log in as buyer — sellers can browse and bid too.
        login(
          {
            user_id:  payload?.user_id  ?? '',
            username: payload?.username ?? email.split('@')[0],
            email,
            role,
          },
          token,
        )
        navigate(role === 'seller' ? '/seller/dashboard' : '/')
      } else {
        await api.auth.register(username, email, password, role)
        navigate('/login')
      }
    } catch (err) {
      if (err instanceof ApiError && err.details?.error === 'username_mismatch') {
        setUpgradePrompt({ existingUsername: err.details.existing_username as string })
      } else {
        setError(err instanceof Error ? err.message : 'Something went wrong')
      }
    } finally {
      setLoading(false)
    }
  }

  const handleUpgradeConfirm = async (chosenUsername: string) => {
    setError(null)
    setLoading(true)
    try {
      await api.auth.register(chosenUsername, email, password, 'seller', true)
      navigate('/login')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
      setUpgradePrompt(null)
    }
  }

  return (
    <PageContainer>
      <div className="max-w-md mx-auto mt-8">
        <Card padding="p-8">
          <h1 className="font-display text-4xl text-brand text-center mb-2">SurpriseAuction</h1>
          <p className="text-center text-text-secondary text-base mb-3">
            {isLogin ? 'Welcome back' : 'Create your account'}
          </p>

          {/* Role toggle — shown on both login and register */}
          <div className="flex rounded-lg border-2 border-border overflow-hidden mb-6">
            {(['buyer', 'seller'] as const).map((r) => (
              <button
                key={r}
                type="button"
                onClick={() => setRole(r)}
                className={`flex-1 py-2.5 text-sm font-semibold transition-colors ${
                  role === r
                    ? 'bg-brand text-white'
                    : 'text-text-secondary hover:text-text-primary'
                }`}
              >
                {r === 'buyer' ? '🛒 Buyer' : '🏪 Seller'}
              </button>
            ))}
          </div>

          {error && (
            <div className="mb-4">
              <StatusBanner type="error" message={error} />
            </div>
          )}

          {upgradePrompt ? (
            <div className="flex flex-col gap-4">
              <p className="text-text-secondary text-sm">
                You already have a buyer account as{' '}
                <span className="font-semibold text-text-primary">{upgradePrompt.existingUsername}</span>.
                You entered{' '}
                <span className="font-semibold text-text-primary">{username}</span>.
                Which username would you like to use?
              </p>
              <div className="flex gap-3">
                <Button
                  variant="outline"
                  size="lg"
                  fullWidth
                  disabled={loading}
                  onClick={() => handleUpgradeConfirm(upgradePrompt.existingUsername)}
                >
                  Keep {upgradePrompt.existingUsername}
                </Button>
                <Button
                  variant="primary"
                  size="lg"
                  fullWidth
                  disabled={loading}
                  onClick={() => handleUpgradeConfirm(username)}
                >
                  Use {username}
                </Button>
              </div>
              <button
                type="button"
                className="text-sm text-text-secondary hover:text-text-primary underline"
                onClick={() => setUpgradePrompt(null)}
              >
                Cancel
              </button>
            </div>
          ) : (
          <>
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
              {loading ? 'Please wait…' : isLogin ? `Sign In as ${role === 'seller' ? 'Seller' : 'Buyer'}` : `Create ${role === 'seller' ? 'Seller' : 'Buyer'} Account`}
            </Button>
          </form>

          <p className="text-center text-base text-text-secondary mt-6">
            {isLogin ? "Don't have an account? " : 'Already have an account? '}
            <Link
              to={isLogin ? '/register' : '/login'}
              className="text-brand font-medium hover:underline"
            >
              {isLogin ? 'Register' : 'Sign In'}
            </Link>
          </p>
          </>
          )}
        </Card>
      </div>
    </PageContainer>
  )
}
