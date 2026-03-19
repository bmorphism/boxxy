import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BoxxyCanvas } from './BoxxyCanvas'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BoxxyCanvas />
  </StrictMode>
)
