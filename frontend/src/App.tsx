import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from '@/context/AuthContext'
import { Navbar } from '@/components/layout'
import HomePage          from '@/pages/HomePage'
import AuctionDetailPage from '@/pages/AuctionDetailPage'
import AuthPage          from '@/pages/AuthPage'
import MyBidsPage        from '@/pages/MyBidsPage'
import ShopDetailPage    from '@/pages/ShopDetailPage'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <div className="min-h-screen flex flex-col font-sans selection:bg-brand/20">
          <Navbar />
          <main className="flex-1">
            <Routes>
              <Route path="/"            element={<HomePage />} />
              <Route path="/auction/:id" element={<AuctionDetailPage />} />
              <Route path="/login"       element={<AuthPage type="login" />} />
              <Route path="/register"    element={<AuthPage type="register" />} />
              <Route path="/my-bids"     element={<MyBidsPage />} />
              <Route path="/shop/:id"    element={<ShopDetailPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </AuthProvider>
  )
}
