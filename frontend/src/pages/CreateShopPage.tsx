import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Button, FormField, TextInput, StatusBanner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

export default function CreateShopPage() {
  const { user, token, isSeller } = useAuth()
  const navigate        = useNavigate()

  const [name,     setName]     = useState('')
  const [location, setLocation] = useState('')
  const [logoUrl,  setLogoUrl]  = useState('')
  const [loading,  setLoading]  = useState(false)
  const [error,    setError]    = useState<string | null>(null)

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to register a shop"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const shop = await api.shops.create({ name, location, logo_url: logoUrl || undefined }, token!)
      navigate(`/shop/${shop.shop_id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer narrow>
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> All Auctions
      </Link>

      <Card padding="p-8">
        <h1 className="font-display text-3xl text-text-primary mb-2">Register Your Shop</h1>
        <p className="text-text-secondary text-sm mb-8">
          Create a shop to start listing surplus food for auction.
        </p>

        {error && (
          <div className="mb-4">
            <StatusBanner type="error" message={error} />
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormField label="Shop Name">
            <TextInput
              type="text"
              required
              placeholder="Le Petit Bakery"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </FormField>

          <FormField label="Location">
            <TextInput
              type="text"
              required
              placeholder="Paris, 2nd arrondissement"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
            />
          </FormField>

          <FormField label="Logo URL (optional)">
            <TextInput
              type="url"
              placeholder="https://example.com/logo.png"
              value={logoUrl}
              onChange={(e) => setLogoUrl(e.target.value)}
            />
          </FormField>

          <Button variant="primary" size="lg" type="submit" fullWidth disabled={loading} className="mt-2">
            {loading ? 'Creating…' : 'Create Shop'}
          </Button>
        </form>
      </Card>
    </PageContainer>
  )
}
