import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Button, FormField, TextInput, TextArea, StatusBanner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

export default function CreateItemPage() {
  const { shopId }      = useParams<{ shopId: string }>()
  const { user, token, isSeller } = useAuth()
  const navigate        = useNavigate()

  const [title,       setTitle]       = useState('')
  const [description, setDescription] = useState('')
  const [retailValue, setRetailValue] = useState('')
  const [imageUrl,    setImageUrl]    = useState('')
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to add items"
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
      await api.shops.createItem(
        shopId!,
        {
          title,
          description,
          retail_value: Math.round(parseFloat(retailValue) * 100),
          image_url:    imageUrl || undefined,
        },
        token!,
      )
      navigate(`/shop/${shopId}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer narrow>
      <Link
        to={`/shop/${shopId}`}
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Shop
      </Link>

      <Card padding="p-8">
        <h1 className="font-display text-3xl text-text-primary mb-2">Add Item</h1>
        <p className="text-text-secondary text-sm mb-8">
          List a product that can be auctioned when you have surplus stock.
        </p>

        {error && (
          <div className="mb-4">
            <StatusBanner type="error" message={error} />
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormField label="Item Title">
            <TextInput
              type="text"
              required
              placeholder="Mystery Pastry Box (3 items)"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </FormField>

          <FormField label="Description">
            <TextArea
              rows={3}
              placeholder="Describe the item — contents, freshness, best-before, etc."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </FormField>

          <FormField label="Retail Value ($)">
            <TextInput
              type="number"
              required
              min="0.01"
              step="0.01"
              placeholder="28.00"
              value={retailValue}
              onChange={(e) => setRetailValue(e.target.value)}
            />
          </FormField>

          <FormField label="Image URL (optional)">
            <TextInput
              type="url"
              placeholder="https://example.com/item.png"
              value={imageUrl}
              onChange={(e) => setImageUrl(e.target.value)}
            />
          </FormField>

          <Button variant="primary" size="lg" type="submit" fullWidth disabled={loading} className="mt-2">
            {loading ? 'Saving…' : 'Add Item'}
          </Button>
        </form>
      </Card>
    </PageContainer>
  )
}
