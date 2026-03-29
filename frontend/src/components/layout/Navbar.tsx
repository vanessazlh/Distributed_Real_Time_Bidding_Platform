import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui'
import { UserIcon } from '@/components/icons'

export function Navbar() {
  const { user, isSeller, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  return (
    <nav className="bg-surface border-b border-border sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-8 h-20 flex items-center justify-between">
        <Link
          to="/"
          className="font-display font-bold text-3xl text-brand tracking-tight hover:opacity-80 transition-opacity"
        >
          SurpriseAuction
        </Link>

        <div className="flex items-center gap-5 font-sans font-medium">
          {user ? (
            isSeller ? (
              <>
                <Link to="/seller/dashboard" className="text-text-primary hover:text-brand transition-colors text-sm">
                  My Dashboard
                </Link>
                <button
                  onClick={handleLogout}
                  className="text-text-secondary hover:text-text-primary transition-colors text-sm"
                >
                  Sign Out
                </button>
                <div className="flex items-center gap-2 px-4 py-2 bg-white rounded-lg border border-border shadow-sm">
                  <UserIcon width={18} height={18} />
                  <span className="text-sm">{user.username}</span>
                </div>
              </>
            ) : (
              <>
                <Link to="/my-bids" className="text-text-primary hover:text-brand transition-colors text-sm">
                  My Bids
                </Link>
                <Link to="/my-payments" className="text-text-primary hover:text-brand transition-colors text-sm">
                  Payments
                </Link>
                <button
                  onClick={handleLogout}
                  className="text-text-secondary hover:text-text-primary transition-colors text-sm"
                >
                  Sign Out
                </button>
                <div className="flex items-center gap-2 px-4 py-2 bg-white rounded-lg border border-border shadow-sm">
                  <UserIcon width={18} height={18} />
                  <span className="text-sm">{user.username}</span>
                </div>
              </>
            )
          ) : (
            <>
              <Button variant="outline" size="md" onClick={() => navigate('/shop/login')}>
                Sell
              </Button>
              <Button variant="primary" size="md" onClick={() => navigate('/login')}>
                Sign In
              </Button>
            </>
          )}
        </div>
      </div>
    </nav>
  )
}
