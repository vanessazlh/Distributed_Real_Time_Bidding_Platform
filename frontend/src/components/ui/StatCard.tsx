import { Card } from './Card'

interface StatCardProps {
  label: string
  value: string | number
}

export function StatCard({ label, value }: StatCardProps) {
  return (
    <Card className="flex-1 text-center" padding="p-5">
      <p className="text-text-secondary text-sm mb-1">{label}</p>
      <p className="font-display text-2xl text-text-primary">{value}</p>
    </Card>
  )
}
