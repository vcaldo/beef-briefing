import { useState, useEffect, createContext, useContext } from 'react'
import { Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { api } from './api/client'
import type { Chat } from './types'

// Pages
import DashboardPage from './pages/DashboardPage'
import MessagesPage from './pages/MessagesPage'
import SearchPage from './pages/SearchPage'
import TopicsPage from './pages/TopicsPage'
import UsersPage from './pages/UsersPage'

// Chat context for sharing selected chat across pages
interface ChatContextType {
  chatId: number | null
  setChatId: (id: number) => void
  chats: Chat[]
  currentChat: Chat | null
}

const ChatContext = createContext<ChatContextType>({
  chatId: null,
  setChatId: () => {},
  chats: [],
  currentChat: null,
})

export const useChat = () => useContext(ChatContext)

function App() {
  const [chats, setChats] = useState<Chat[]>([])
  const [chatId, setChatId] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const location = useLocation()

  useEffect(() => {
    async function loadChats() {
      try {
        const response = await api.getChats()
        setChats(response.chats)
        // Auto-select first chat
        if (response.chats.length > 0 && !chatId) {
          setChatId(response.chats[0].id)
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load chats')
      } finally {
        setLoading(false)
      }
    }
    loadChats()
  }, [])

  const currentChat = chats.find((c) => c.id === chatId) || null

  if (loading) {
    return (
      <div className="app-layout">
        <div className="loading">Loading...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="app-layout">
        <div className="main-content">
          <div className="error">
            <strong>Error:</strong> {error}
            <p style={{ marginTop: '8px', fontSize: '13px' }}>
              Make sure the backend is running at {import.meta.env.VITE_API_URL || 'http://localhost:8052'}
            </p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <ChatContext.Provider value={{ chatId, setChatId, chats, currentChat }}>
      <div className="app-layout">
        <aside className="sidebar">
          <div className="sidebar-header">
            <div className="sidebar-title">ML Dashboard</div>
            <div className="sidebar-subtitle">Dev-only exploration tool</div>
          </div>

          <nav className="sidebar-nav">
            <NavLink to="/" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
              <span className="nav-icon">&#x1F4CA;</span>
              Dashboard
            </NavLink>
            <NavLink to="/messages" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
              <span className="nav-icon">&#x1F4AC;</span>
              Messages
            </NavLink>
            <NavLink to="/search" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
              <span className="nav-icon">&#x1F50D;</span>
              Search
            </NavLink>
            <NavLink to="/topics" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
              <span className="nav-icon">&#x1F3F7;</span>
              Topics
            </NavLink>
            <NavLink to="/users" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
              <span className="nav-icon">&#x1F465;</span>
              Users
            </NavLink>
          </nav>

          <div className="sidebar-footer">
            <select
              className="chat-selector"
              value={chatId || ''}
              onChange={(e) => setChatId(Number(e.target.value))}
            >
              {chats.map((chat) => (
                <option key={chat.id} value={chat.id}>
                  {chat.title || `Chat ${chat.id}`}
                </option>
              ))}
            </select>
          </div>
        </aside>

        <main className="main-content">
          <Routes>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/messages" element={<MessagesPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/topics" element={<TopicsPage />} />
            <Route path="/users" element={<UsersPage />} />
          </Routes>
        </main>
      </div>
    </ChatContext.Provider>
  )
}

export default App
