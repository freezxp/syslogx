import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App } from './App'
import './styles.css'
import './phase4.css'
import './login.css'
import './insights.css'

const queryClient = new QueryClient({defaultOptions: {queries: {retry: 2, staleTime: 2_000, refetchOnWindowFocus: false}}})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}><App /></QueryClientProvider>
  </StrictMode>
)
