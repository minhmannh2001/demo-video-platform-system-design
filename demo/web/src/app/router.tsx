import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { HomeView } from '@/views/home'
import { UploadDetailView } from '@/views/upload-detail'
import { UploadsListView } from '@/views/uploads-list'
import { UploadView } from '@/views/upload'
import { VideoWatchView } from '@/views/video-watch'

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<HomeView />} />
        <Route path="/upload" element={<UploadView />} />
        <Route path="/uploads/:id" element={<UploadDetailView />} />
        <Route path="/uploads" element={<UploadsListView />} />
        <Route path="/watch/:id" element={<VideoWatchView />} />
      </Routes>
    </BrowserRouter>
  )
}
