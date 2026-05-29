import { BrowserRouter, Routes, Route } from 'react-router-dom'
import RootPage from './pages/RootPage'
import WorkspacePage from './pages/WorkspacePage'
import CommitPage from './pages/CommitPage'
import TestEditorPage from './pages/TestEditorPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RootPage />} />
        <Route path="/:name" element={<WorkspacePage />} />
        <Route path="/:name/commits/:commitId" element={<CommitPage />} />
        <Route path="/debug/editor" element={<TestEditorPage />} />
      </Routes>
    </BrowserRouter>
  )
}
