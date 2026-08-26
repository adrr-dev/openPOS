import { Link, useLocation, useNavigate } from 'react-router'

export default function Navbar({ dark }: { dark?: boolean }) {
  const nav = useNavigate()
  const loc = useLocation()

  function goHome(e: React.MouseEvent) {
    e.preventDefault()
    if (loc.pathname === '/') {
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } else {
      nav('/')
    }
  }

  function goSection(e: React.MouseEvent, id: string) {
    e.preventDefault()
    if (loc.pathname === '/') {
      document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    } else {
      nav('/')
      setTimeout(() => document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 120)
    }
  }

  return (
    <header className="sticky top-0 z-10 border-b border-border bg-bg/90 backdrop-blur-xl">
      <div className="mx-auto grid max-w-6xl grid-cols-[1fr_auto_1fr] items-center gap-5 px-8 py-3.5">
        <Link to="/" onClick={goHome} className="flex items-center justify-self-start">
          <img src="/logo.png" alt="OpenPOS" className="h-7 w-auto" />
        </Link>
        {!dark && (
          <nav className="hidden gap-8 text-sm text-muted md:flex" aria-label="Navigasi utama">
            <button onClick={(e) => goSection(e, 'fitur')} className="hover:text-jet">Fitur</button>
            <button onClick={(e) => goSection(e, 'cara-kerja')} className="hover:text-jet">Cara Kerja</button>
            <button onClick={(e) => goSection(e, 'tentang')} className="hover:text-jet">Tentang</button>
          </nav>
        )}
        <div className="flex items-center justify-self-end gap-2.5">
          <Link to="/masuk" className="rounded-full border border-dove px-4 py-2 text-sm font-medium hover:border-jet">Masuk</Link>
          <Link to="/daftar" className="rounded-full border border-jet px-4 py-2 text-sm font-medium hover:bg-jet hover:text-paper">Mulai Gratis</Link>
        </div>
      </div>
    </header>
  )
}