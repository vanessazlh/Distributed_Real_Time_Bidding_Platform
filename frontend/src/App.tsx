import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from '@/context/AuthContext'
import { NotificationProvider } from '@/context/NotificationContext'
import { WatchlistProvider } from '@/context/WatchlistContext'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Navbar } from '@/components/layout'
import { NotificationToast } from '@/components/ui/Toast'
import HomePage          from '@/pages/HomePage'
import AuctionDetailPage from '@/pages/AuctionDetailPage'
import AuthPage          from '@/pages/AuthPage'
import ShopDetailPage    from '@/pages/ShopDetailPage'
import CreateShopPage    from '@/pages/CreateShopPage'
import CreateItemPage    from '@/pages/CreateItemPage'
import CreateAuctionPage from '@/pages/CreateAuctionPage'
import SellerDashboardPage    from '@/pages/SellerDashboardPage'
import SellerShopPage              from '@/pages/SellerShopPage'
import SellerAuctionDetailPage    from '@/pages/SellerAuctionDetailPage'
import PaymentPage            from '@/pages/PaymentPage'
import ProfilePage            from '@/pages/ProfilePage'
import WatchlistPage          from '@/pages/WatchlistPage'

export default function App() {
  return (
    <ErrorBoundary>
    <AuthProvider>
      <BrowserRouter>
        <NotificationProvider>
        <WatchlistProvider>
        <div className="min-h-screen flex flex-col font-sans selection:bg-brand/20">
          <Navbar />
          <NotificationToast />
          <main className="flex-1">
            <Routes>
              <Route path="/"                          element={<HomePage />} />
              <Route path="/auction/:id"               element={<AuctionDetailPage />} />
              <Route path="/login"                     element={<AuthPage type="login" />} />
              <Route path="/register"                  element={<AuthPage type="register" />} />
              <Route path="/profile"                    element={<ProfilePage />} />
              <Route path="/profile/:tab"               element={<ProfilePage />} />
              <Route path="/watchlist"                  element={<WatchlistPage />} />
              <Route path="/shop/:id"                  element={<ShopDetailPage />} />
              <Route path="/shops/new"                 element={<CreateShopPage />} />
              <Route path="/shops/:shopId/items/new"   element={<CreateItemPage />} />
              <Route path="/auctions/new"              element={<CreateAuctionPage />} />
              <Route path="/shop/login"               element={<Navigate to="/login" replace />} />
              <Route path="/shop/register"            element={<Navigate to="/register" replace />} />
              <Route path="/seller/dashboard"              element={<SellerDashboardPage />} />
              <Route path="/seller/auctions/:auctionId"      element={<SellerAuctionDetailPage />} />
              <Route path="/seller/shops/:shopId/:tab"     element={<SellerShopPage />} />
              <Route path="/seller/shops/:shopId"          element={<Navigate to="items" replace />} />
              <Route path="/payment/auction/:auctionId"   element={<PaymentPage />} />
            </Routes>
          </main>
        </div>
        </WatchlistProvider>
        </NotificationProvider>
      </BrowserRouter>
    </AuthProvider>
    </ErrorBoundary>
  )
}
