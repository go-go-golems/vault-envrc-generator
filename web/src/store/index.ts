import { configureStore } from '@reduxjs/toolkit'
import vaultSlice from './vaultSlice'

export const store = configureStore({
  reducer: {
    vault: vaultSlice,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
