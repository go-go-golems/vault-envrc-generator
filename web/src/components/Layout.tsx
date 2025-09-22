import React from 'react'
import { NavLink } from 'react-router-dom'
import ConnectionStatus from './ConnectionStatus'

interface LayoutProps {
  children: React.ReactNode
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  return (
    <div className="d-flex" style={{ height: '100vh' }}>
      {/* Sidebar */}
      <div className="bg-dark text-white" style={{ width: '250px', minHeight: '100vh' }}>
        <div className="p-3 border-bottom border-secondary">
          <h5 className="mb-0 d-flex align-items-center">
            <i className="bi bi-shield-lock me-2"></i>
            Vault Envrc Generator
          </h5>
        </div>
        
        <nav className="p-3">
          <div className="nav nav-pills flex-column">
            <NavLink 
              to="/" 
              className={({ isActive }) => 
                `nav-link d-flex align-items-center mb-2 ${isActive ? 'active' : 'text-white-50'}`
              }
            >
              <i className="bi bi-house me-2"></i>
              Dashboard
            </NavLink>
            
            <NavLink 
              to="/explorer" 
              className={({ isActive }) => 
                `nav-link d-flex align-items-center mb-2 ${isActive ? 'active' : 'text-white-50'}`
              }
            >
              <i className="bi bi-tree me-2"></i>
              Explorer
            </NavLink>
            
            <NavLink 
              to="/generator" 
              className={({ isActive }) => 
                `nav-link d-flex align-items-center mb-2 ${isActive ? 'active' : 'text-white-50'}`
              }
            >
              <i className="bi bi-file-earmark-code me-2"></i>
              Generator
            </NavLink>
            
            <NavLink 
              to="/settings" 
              className={({ isActive }) => 
                `nav-link d-flex align-items-center mb-2 ${isActive ? 'active' : 'text-white-50'}`
              }
            >
              <i className="bi bi-gear me-2"></i>
              Settings
            </NavLink>
          </div>
        </nav>
        
        <div className="mt-auto p-3 border-top border-secondary">
          <ConnectionStatus />
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-grow-1 d-flex flex-column">
        {/* Header */}
        <header className="bg-white border-bottom px-4 py-3">
          <div className="d-flex justify-content-between align-items-center">
            <h2 className="mb-0 text-dark">
              <PageTitle />
            </h2>
            <div className="d-flex align-items-center gap-3">
              <QuickActions />
            </div>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-grow-1 p-4 bg-light overflow-auto">
          {children}
        </main>
      </div>
    </div>
  )
}

const PageTitle = () => {
  const path = window.location.pathname
  const titles: Record<string, string> = {
    '/': 'Dashboard',
    '/explorer': 'Vault Explorer',
    '/generator': 'File Generator',
    '/settings': 'Settings'
  }
  return <>{titles[path] || 'Vault Envrc Generator'}</>
}

const QuickActions = () => {
  return (
    <div className="d-flex align-items-center gap-2">
      <button className="btn btn-outline-primary btn-sm" title="Refresh All">
        <i className="bi bi-arrow-clockwise"></i>
      </button>
      <button className="btn btn-outline-secondary btn-sm" title="Help">
        <i className="bi bi-question-circle"></i>
      </button>
    </div>
  )
}

export default Layout


