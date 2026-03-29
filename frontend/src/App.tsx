import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from '@/context/AuthContext'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Navbar } from '@/components/layout'
import HomePage          from '@/pages/HomePage'
import AuctionDetailPage from '@/pages/AuctionDetailPage'
import AuthPage          from '@/pages/AuthPage'
import MyBidsPage        from '@/pages/MyBidsPage'
import ShopDetailPage    from '@/pages/ShopDetailPage'
import CreateShopPage    from '@/pages/CreateShopPage'
import CreateItemPage    from '@/pages/CreateItemPage'
import CreateAuctionPage from '@/pages/CreateAuctionPage'
import ShopAuthPage           from '@/pages/ShopAuthPage'
import SellerDashboardPage    from '@/pages/SellerDashboardPage'
import PaymentPage            from '@/pages/PaymentPage'
import MyPaymentsPage         from '@/pages/MyPaymentsPage'

export default function App() {
  return (
    <ErrorBoundary>
    <AuthProvider>
      <BrowserRouter>
        <div className="min-h-screen flex flex-col font-sans selection:bg-brand/20">
          <Navbar />
          <main className="flex-1">
            <Routes>
              <Route path="/"                          element={<HomePage />} />
              <Route path="/auction/:id"               element={<AuctionDetailPage />} />
              <Route path="/login"                     element={<AuthPage type="login" />} />
              <Route path="/register"                  element={<AuthPage type="register" />} />
              <Route path="/my-bids"                   element={<MyBidsPage />} />
              <Route path="/shop/:id"                  element={<ShopDetailPage />} />
              <Route path="/shops/new"                 element={<CreateShopPage />} />
              <Route path="/shops/:shopId/items/new"   element={<CreateItemPage />} />
              <Route path="/auctions/new"              element={<CreateAuctionPage />} />
              <Route path="/shop/login"               element={<ShopAuthPage type="login" />} />
              <Route path="/shop/register"            element={<ShopAuthPage type="register" />} />
              <Route path="/seller/dashboard"              element={<SellerDashboardPage />} />
              <Route path="/my-payments"                  element={<MyPaymentsPage />} />
              <Route path="/payment/auction/:auctionId"   element={<PaymentPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </AuthProvider>
    </ErrorBoundary>
  )
}
