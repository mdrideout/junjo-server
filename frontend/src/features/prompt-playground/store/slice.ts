import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'

interface PromptPlaygroundState {
  output: string | null
  loading: boolean
  error: string | null
  selectedModel: string | null
  jsonMode: boolean
}

const initialState: PromptPlaygroundState = {
  output: null,
  loading: false,
  error: null,
  selectedModel: null,
  jsonMode: false,
}

export const promptPlaygroundSlice = createSlice({
  name: 'promptPlayground',
  initialState,
  reducers: {
    setOutput: (state, action: PayloadAction<string | null>) => {
      state.output = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
    setError: (state, action: PayloadAction<string | null>) => {
      state.error = action.payload
    },
    setSelectedModel: (state, action: PayloadAction<string | null>) => {
      state.selectedModel = action.payload
    },
    setJsonMode: (state, action: PayloadAction<boolean>) => {
      state.jsonMode = action.payload
    },
  },
})

export const PromptPlaygroundActions = promptPlaygroundSlice.actions
export default promptPlaygroundSlice.reducer
