import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui'
import { UserIcon, HeartIcon } from '@/components/icons'
import { NotificationBell } from './NotificationBell'

export function Navbar() {
  const { user, isSeller, logout } = useAuth()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)

  const handleLogout = () => {
    logout()
    navigate('/')
    setMenuOpen(false)
  }

  return (
    <nav className="bg-surface border-b border-border sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 md:px-8 h-16 md:h-20 flex items-center justify-between">
        <Link
          to="/"
          className="font-display font-bold text-2xl md:text-3xl text-brand tracking-tight hover:opacity-80 transition-opacity"
        >
          SurpriseAuction
        </Link>

        {/* Desktop nav */}
        <div className="hidden sm:flex items-center gap-5 font-sans font-medium">
          {user ? (
            isSeller ? (
              <>
                <Link to="/seller/dashboard" className="text-text-primary hover:text-brand transition-colors text-base">
                  My Dashboard
                </Link>
                <button
                  onClick={handleLogout}
                  className="text-text-secondary hover:text-text-primary transition-colors text-base"
                >
                  Sign Out
                </button>
                <Link to="/profile" className="flex items-center gap-2 px-4 py-2 bg-white rounded-lg border border-border shadow-sm hover:border-brand transition-colors">
                  <UserIcon width={18} height={18} />
                  <span className="text-base">{user.username}</span>
                </Link>
              </>
            ) : (
              <>
                <Link
                  to="/watchlist"
                  className="text-text-secondary hover:text-red-500 transition-colors"
                  title="My Watchlist"
                >
                  <HeartIcon width={20} height={20} />
                </Link>
                <NotificationBell />
                <button
                  onClick={handleLogout}
                  className="text-text-secondary hover:text-text-primary transition-colors text-base"
                >
                  Sign Out
                </button>
                <Link to="/profile" className="flex items-center gap-2 px-4 py-2 bg-white rounded-lg border border-border shadow-sm hover:border-brand transition-colors">
                  <UserIcon width={18} height={18} />
                  <span className="text-base">{user.username}</span>
                </Link>
              </>
            )
          ) : (
            <Button variant="primary" size="md" onClick={() => navigate('/login')}>
              Sign In
            </Button>
          )}
        </div>

        {/* Mobile right side: notification + hamburger */}
        <div className="flex sm:hidden items-center gap-3">
          {user && !isSeller && <NotificationBell />}
          <button
            onClick={() => setMenuOpen((o) => !o)}
            aria-label="Toggle menu"
            className="p-2 rounded-lg text-text-secondary hover:text-text-primary hover:bg-surface-alt transition-colors"
          >
            {menuOpen ? (
              <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            ) : (
              <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Mobile dropdown menu */}
      {menuOpen && (
        <div className="sm:hidden border-t border-border bg-surface px-4 py-3 flex flex-col gap-1 font-sans font-medium">
          {user ? (
            <>
              <div className="flex items-center gap-3 px-3 py-2 mb-1">
                <UserIcon width={18} height={18} className="text-brand" />
                <span className="text-text-primary font-semibold">{user.username}</span>
              </div>
              {isSeller ? (
                <Link
                  to="/seller/dashboard"
                  onClick={() => setMenuOpen(false)}
                  className="px-3 py-2.5 rounded-lg text-text-primary hover:bg-brand/5 hover:text-brand transition-colors"
                >
                  My Dashboard
                </Link>
              ) : (
                <>
                  <Link
                    to="/watchlist"
                    onClick={() => setMenuOpen(false)}
                    className="px-3 py-2.5 rounded-lg text-text-primary hover:bg-brand/5 hover:text-brand transition-colors"
                  >
                    My Watchlist
                  </Link>
                </>
              )}
              <Link
                to="/profile"
                onClick={() => setMenuOpen(false)}
                className="px-3 py-2.5 rounded-lg text-text-primary hover:bg-brand/5 hover:text-brand transition-colors"
              >
                Profile
              </Link>
              <button
                onClick={handleLogout}
                className="text-left px-3 py-2.5 rounded-lg text-text-secondary hover:bg-brand/5 hover:text-text-primary transition-colors"
              >
                Sign Out
              </button>
            </>
          ) : (
            <div className="py-2">
              <Button variant="primary" size="md" fullWidth onClick={() => { navigate('/login'); setMenuOpen(false) }}>
                Sign In
              </Button>
            </div>
          )}
        </div>
      )}
    </nav>
  )
}
