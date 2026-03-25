import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { HomeView } from '@/views/home'
import { UploadView } from '@/views/upload'
import { VideoWatchView } from '@/views/video-watch'

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<HomeView />} />
        <Route path="/upload" element={<UploadView />} />
        <Route path="/watch/:id" element={<VideoWatchView />} />
      </Routes>
    </BrowserRouter>
  )
}
