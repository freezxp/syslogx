import type { LucideIcon } from 'lucide-react'
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  Activity, Boxes, ChevronRight, CircleGauge, Database, FileSearch, Filter, Gauge,
  HardDrive, LayoutDashboard, Menu, Radio, RefreshCw, Search, Server, Settings,
  ShieldCheck, TerminalSquare, X
} from 'lucide-react'
import { api, range, type Filter as QueryFilter, type LogQuery, type LogRow, type ValueCount, value } from './api'

const VolumeChart = lazy(() => import("./VolumeChart"))

type Page = 'dashboard' | 'logs' | 'live' | 'sources' | 'searches' | 'system'

type NavItem = {section: string} | {label: string; page: Page; icon: LucideIcon; soon?: boolean}

const nav: NavItem[] = [
  {label: 'Dashboard', page: 'dashboard' as Page, icon: LayoutDashboard},
  {section: 'EXPLORE'},
  {label: 'Logs', page: 'logs' as Page, icon: FileSearch},
  {label: 'Live tail', page: 'live' as Page, icon: Radio},
  {section: 'MANAGE'},
  {label: 'Sources', page: 'sources' as Page, icon: Boxes},
  {label: 'Saved searches', page: 'searches' as Page, icon: Search},
  {label: 'System', page: 'system' as Page, icon: CircleGauge}
]

export function App() {
  const [page, setPage] = useState<Page>('dashboard')
  const [menuOpen, setMenuOpen] = useState(false)
  const logs = useQuery({queryKey: ['recent-logs'], queryFn: ({signal}) => api.search({...range('15m'),limit:1000},signal), refetchInterval: 5_000})
  const ready = useQuery({queryKey: ['ready'], queryFn: ({signal}) => api.ready(signal), refetchInterval: 5_000})
  const identity = useQuery({queryKey:['identity'],queryFn:({signal})=>api.me(signal),retry:false})
  const rows = logs.data?.data ?? []
  const navigate = (next: Page) => { setPage(next); setMenuOpen(false) }

  if(identity.isError)return <Login onSuccess={()=>identity.refetch()}/>
  return <div className="app-shell">
    <aside className={menuOpen ? 'sidebar open' : 'sidebar'}>
      <div className="brand"><div className="brand-mark"><TerminalSquare size={19}/></div><div><strong>SYSLOGX</strong><span>OBSERVABILITY</span></div></div>
      <nav>{nav.map((item, index) => 'section' in item
        ? <div className="nav-section" key={item.section}>{item.section}</div>
        : <button key={item.label} className={page === item.page ? 'nav-item active' : 'nav-item'} onClick={() => navigate(item.page!)}>
            <item.icon size={17}/><span>{item.label}</span>{item.soon && <small>SOON</small>}
          </button>)}</nav>
      <div className="sidebar-bottom"><div className={ready.data?.status === 'ready' ? 'health-dot good' : 'health-dot'}/><div><strong>{ready.data?.status === 'ready' ? 'All systems operational' : 'System needs attention'}</strong><span>Phase 4 · VictoriaLogs</span></div></div>
    </aside>
    {menuOpen && <button className="scrim" aria-label="Close navigation" onClick={() => setMenuOpen(false)}/>} 
    <main>
      <header className="topbar"><button className="icon-button mobile-menu" onClick={() => setMenuOpen(true)}><Menu size={19}/></button><div className="crumb"><span>Syslogx</span><ChevronRight size={14}/><strong>{pageTitle(page)}</strong></div><div className="top-actions"><div className="phase-pill"><span/> PHASE 4</div><button className="icon-button" aria-label="Settings"><Settings size={18}/></button><div className="avatar">SX</div></div></header>
      {page === 'dashboard' && <Dashboard rows={rows} loading={logs.isLoading} error={logs.error} onRefresh={() => logs.refetch()} ready={ready.data}/>} 
      {page === 'logs' && <LogExplorer/>}
      {page === 'live' && <LiveTail/>}
      {page === 'sources' && <Sources/>}
      {page === 'searches' && <SavedSearches onOpen={(q)=>{history.replaceState(null,'',`/logs?query=${encodeURIComponent(q)}`);navigate('logs')}}/>}
      {page === 'system' && <SystemView ready={ready.data}/>} 
    </main>
  </div>
}

function Login({onSuccess}:{onSuccess:()=>void}){const[user,setUser]=useState('admin');const[password,setPassword]=useState('');const[error,setError]=useState('');const submit=async(e:React.FormEvent)=>{e.preventDefault();try{await api.login(user,password);onSuccess()}catch{setError('Invalid username or password')}};return <main className="login-page"><form onSubmit={submit}><div className="brand-mark"><TerminalSquare size={19}/></div><span className="eyebrow">SYSLOGX</span><h1>Welcome back</h1><p>Sign in to search and analyze your logs.</p><label>Username<input autoFocus value={user} onChange={e=>setUser(e.target.value)}/></label><label>Password<input type="password" value={password} onChange={e=>setPassword(e.target.value)}/></label>{error&&<span className="login-error">{error}</span>}<button>Sign in</button></form></main>}

function pageTitle(page: Page) { return ({dashboard:'Dashboard',logs:'Log explorer',live:'Live tail',sources:'Sources',searches:'Saved searches',system:'System health'})[page] }

function Dashboard({rows, loading, error, onRefresh, ready}: {rows:LogRow[];loading:boolean;error:Error|null;onRefresh:()=>void;ready?:{status:string}}) {
  const stats = useMemo(() => deriveStats(rows), [rows])
  return <div className="page"><PageHeader eyebrow="OVERVIEW" title="System dashboard" description="A live operational view of your bounded 15-minute log window." action={<button className="secondary-button" onClick={onRefresh}><RefreshCw size={15}/> Refresh</button>}/>
    {error && <Notice>VictoriaLogs is unavailable. The UI is running and will reconnect automatically.</Notice>}
    <div className="metric-grid">
      <Metric icon={Database} label="Recent logs" value={loading?'—':formatNumber(rows.length)} detail="Last 15 minutes" tone="violet"/>
      <Metric icon={Gauge} label="Window rate" value={`${stats.rate.toFixed(1)}/s`} detail="Observed result window" tone="cyan"/>
      <Metric icon={ShieldCheck} label="Errors" value={formatNumber(stats.errors)} detail={`${stats.errorPercent.toFixed(1)}% of results`} tone="red"/>
      <Metric icon={Server} label="Active sources" value={String(stats.sources)} detail="Unique source IDs" tone="amber"/>
    </div>
    <div className="dashboard-grid">
      <section className="panel chart-panel"><PanelTitle title="Log volume" subtitle="Events per minute"/><div className="chart-wrap"><Suspense fallback={<div className="loading">Loading chart…</div>}><VolumeChart data={stats.volume}/></Suspense></div></section>
      <section className="panel"><PanelTitle title="Severity" subtitle="Distribution in current window"/><div className="severity-list">{stats.severities.map(s=><div className="severity-row" key={s.name}><div><span className={`severity-dot ${severityClass(s.name)}`}/><strong>{s.name}</strong></div><div className="severity-track"><span style={{width:`${s.percent}%`}} className={severityClass(s.name)}/></div><b>{formatNumber(s.count)}</b></div>)}</div></section>
      <section className="panel sources-panel"><PanelTitle title="Top hosts" subtitle="Most active senders"/><div className="rank-list">{stats.hosts.slice(0,5).map((h,i)=><div key={h.name}><span className="rank">{String(i+1).padStart(2,'0')}</span><div><strong>{h.name}</strong><small>{h.percent.toFixed(1)}% of traffic</small></div><b>{formatNumber(h.count)}</b></div>)}{!stats.hosts.length&&<Empty compact/>}</div></section>
      <section className="panel status-panel"><PanelTitle title="Platform" subtitle="Dependency readiness"/><div className="status-hero"><div className={ready?.status==='ready'?'status-ring good':'status-ring'}><Activity/></div><div><strong>{ready?.status==='ready'?'Ready to ingest':'Waiting for dependencies'}</strong><span>{ready?.status==='ready'?'Syslog listeners and storage are available.':'The UI will recover automatically when services are ready.'}</span></div></div><div className="status-line"><span>Storage engine</span><b>VictoriaLogs</b></div><div className="status-line"><span>Delivery mode</span><b>Bounded memory</b></div></section>
    </div>
  </div>
}

function LogExplorer() {
  const params=new URLSearchParams(location.search)
  const [query,setQuery]=useState(params.get('query')??''); const [duration,setDuration]=useState(params.get('range')??'1h'); const [customStart,setCustomStart]=useState('');const[customEnd,setCustomEnd]=useState(''); const [mode,setMode]=useState<'visual'|'advanced'>('visual'); const [filters,setFilters]=useState<QueryFilter[]>([]); const [selected,setSelected]=useState<LogRow|null>(null); const [cursor,setCursor]=useState(''); const [submitted,setSubmitted]=useState(0);const[clock,setClock]=useState(0)
  useEffect(()=>{const timer=setInterval(()=>setClock(x=>x+1),5000);return()=>clearInterval(timer)},[])
  const times=useMemo(()=>duration==='custom'&&customStart&&customEnd?{start:new Date(customStart).toISOString(),end:new Date(customEnd).toISOString()}:range(duration),[duration,customStart,customEnd,submitted,clock]); const spec:LogQuery={...times,query:mode==='visual'?query:undefined,native_query:mode==='advanced'?query:undefined,filters,limit:250,cursor}
  const logs=useQuery({queryKey:['search',spec,submitted],queryFn:({signal})=>api.search(spec,signal),placeholderData:previous=>previous})
  const fields=useQuery({queryKey:['fields',times],queryFn:({signal})=>api.fields({...times,limit:500},signal)})
  const stats=useQuery({queryKey:['explorer-stats',spec,submitted],queryFn:({signal})=>api.stats({...spec,cursor:'',limit:1000},signal)})
  const facetGroups:{field:string;items:ValueCount[]}[]=[{field:'severity_name',items:stats.data?.severities??[]},{field:'hostname',items:stats.data?.top_hosts??[]}]
  const run=()=>{setCursor('');history.replaceState(null,'',`/logs?range=${encodeURIComponent(duration)}&query=${encodeURIComponent(query)}`);setSubmitted(x=>x+1)}
  const save=async()=>{const name=prompt('Saved search name');if(name)await api.save({name,description:'',query,default_time_range:duration})}
  const exportLogs=async(format:string)=>{const response=await fetch(`/api/v1/logs/export?format=${format}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...spec,limit:1000})});if(!response.ok)return;const blob=await response.blob();const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=`syslogx.${format}`;a.click();URL.revokeObjectURL(a.href)}
  return <div className="page logs-page"><PageHeader eyebrow="EXPLORE" title="Log explorer" description="Search structured events with server-side filters and cursor pagination." action={<><button className="secondary-button" onClick={save}>Save</button><button className="secondary-button" onClick={()=>logs.refetch()}><RefreshCw size={15}/> Refresh</button></>}/>
    <div className="query-toolbar"><div className="search-box"><Search size={17}/><input value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>e.key==='Enter'&&run()} placeholder={mode==='advanced'?'Enter LogsQL…':'Search log messages…'}/><kbd>/</kbd></div><select value={duration} onChange={e=>setDuration(e.target.value)}>{['5m','15m','30m','1h','3h','6h','12h','24h','7d','30d','custom'].map(v=><option key={v}>{v}</option>)}</select><button className="secondary-button" onClick={run}>Run</button></div>
    {duration==='custom'&&<div className="custom-range"><label>Start <input type="datetime-local" value={customStart} onChange={e=>setCustomStart(e.target.value)}/></label><label>End <input type="datetime-local" value={customEnd} onChange={e=>setCustomEnd(e.target.value)}/></label><span>Browser timezone</span></div>}
    <div className="results-meta"><span><button className="text-button" onClick={()=>setMode(mode==='visual'?'advanced':'visual')}>{mode==='visual'?'Visual query':'Advanced LogsQL'}</button> · <b>{formatNumber(logs.data?.data.length??0)}</b> records</span><span className="bounded-label">PHASE 4 EXPLORER</span></div>
    {mode==='visual'&&<div className="filter-bar">{filters.map((f,i)=><div className="filter-chip" key={i}><select value={f.field} onChange={e=>setFilters(filters.map((x,n)=>n===i?{...x,field:e.target.value}:x))}>{(fields.data?.data??[]).map(x=><option key={x.name}>{x.name}</option>)}</select><select value={f.operator} onChange={e=>setFilters(filters.map((x,n)=>n===i?{...x,operator:e.target.value}:x))}>{['eq','neq','contains','starts_with','exists','gt','lt'].map(x=><option key={x}>{x}</option>)}</select>{f.operator!=='exists'&&<input value={f.value} onChange={e=>setFilters(filters.map((x,n)=>n===i?{...x,value:e.target.value}:x))}/>}<button onClick={()=>setFilters(filters.filter((_,n)=>n!==i))}><X size={13}/></button></div>)}<button className="secondary-button" onClick={()=>setFilters([...filters,{field:'hostname',operator:'eq',value:''}])}>+ Filter</button></div>}
    <div className="explorer-insights"><section className="panel"><PanelTitle title="Log volume" subtitle="Current query"/><div className="mini-chart"><Suspense fallback={<div className="loading">Loading…</div>}><VolumeChart data={(stats.data?.volume??[]).map(x=>({label:new Date(x.timestamp).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'}),count:x.count}))}/></Suspense></div></section><section className="panel facets"><PanelTitle title="Facets" subtitle="Click to filter"/>{facetGroups.map(({field,items})=><div className="facet-group" key={field}><strong>{field}</strong>{items.slice(0,6).map(v=><button key={v.value} onClick={()=>setFilters([...filters,{field,operator:'eq',value:v.value}])}><span>{v.value}</span><b>{v.count}</b></button>)}</div>)}</section></div>
    <div className="field-strip"><span>Fields</span>{(fields.data?.data??[]).slice(0,10).map(f=><button key={f.name} onClick={()=>setFilters([...filters,{field:f.name,operator:'exists'}])}>{f.name}<b>{f.count}</b></button>)}<span className="export-actions">Export <button onClick={()=>exportLogs('json')}>JSON</button><button onClick={()=>exportLogs('csv')}>CSV</button><button onClick={()=>exportLogs('ndjson')}>NDJSON</button></span></div>
    {logs.error&&<Notice>{logs.error.message}</Notice>}<LogTable rows={logs.data?.data??[]} loading={logs.isLoading} onSelect={setSelected}/>{logs.data?.next_cursor&&<button className="secondary-button load-more" onClick={()=>setCursor(logs.data!.next_cursor!)}>Load next page</button>}
    {selected&&<Detail row={selected} onClose={()=>setSelected(null)}/>} 
  </div>
}

function LiveTail(){const[rows,setRows]=useState<LogRow[]>([]);const[paused,setPaused]=useState(false);const[query,setQuery]=useState('');useEffect(()=>{if(paused)return;const source=new EventSource(`/api/v1/logs/tail?query=${encodeURIComponent(query)}`);source.onmessage=e=>{const row=JSON.parse(e.data) as LogRow;setRows(old=>[row,...old].slice(0,500))};return()=>source.close()},[paused,query]);return <div className="page logs-page"><PageHeader eyebrow="LIVE" title="Live tail" description="Streaming events over Server-Sent Events." action={<><button className="secondary-button" onClick={()=>setPaused(!paused)}>{paused?'Resume':'Pause'}</button><button className="secondary-button" onClick={()=>setRows([])}>Clear</button></>}/><div className="query-toolbar"><div className="search-box"><Radio size={17}/><input value={query} onChange={e=>setQuery(e.target.value)} placeholder="Filter incoming logs…"/></div><div className="time-chip">{paused?'Paused':'Connected'}</div></div><LogTable rows={rows} loading={false} onSelect={()=>{}}/></div>}

function LogTable({rows,loading,onSelect}:{rows:LogRow[];loading:boolean;onSelect:(r:LogRow)=>void}) {
  const parent=useRef<HTMLDivElement>(null)
  const columns=useMemo<ColumnDef<LogRow>[]>(()=>['timestamp','hostname','severity_name','app_name','message'].map(id=>({id,accessorFn:(row)=>value(row,id)})),[])
  const table=useReactTable({data:rows,columns,getCoreRowModel:getCoreRowModel()})
  const tableRows=table.getRowModel().rows
  const virtual=useVirtualizer({count:tableRows.length,getScrollElement:()=>parent.current,estimateSize:()=>43,overscan:12})
  if(loading)return <div className="table-shell"><div className="loading"><RefreshCw className="spin"/> Loading logs</div></div>
  if(!rows.length)return <div className="table-shell"><Empty/></div>
  return <div className="table-shell"><div className="log-head"><span>Timestamp</span><span>Host</span><span>Severity</span><span>Application</span><span>Message</span></div><div className="log-scroll" ref={parent}><div style={{height:virtual.getTotalSize(),position:'relative'}}>{virtual.getVirtualItems().map(item=>{const row=tableRows[item.index].original;return <button className="log-row" key={`${value(row,'id')}-${item.index}`} style={{transform:`translateY(${item.start}px)`}} onClick={()=>onSelect(row)}><time>{formatTime(value(row,'_time')!=='—'?value(row,'_time'):value(row,'timestamp'))}</time><span className="host">{value(row,'hostname')}</span><span><em className={`severity-badge ${severityClass(value(row,'severity_name'))}`}>{value(row,'severity_name')}</em></span><span>{value(row,'app_name')}</span><span className="message">{value(row,'_msg')!=='—'?value(row,'_msg'):value(row,'message')}</span></button>})}</div></div></div>
}

function Detail({row,onClose}:{row:LogRow;onClose:()=>void}) { const entries=Object.entries(row).filter(([k])=>k!=='_msg').sort(([a],[b])=>a.localeCompare(b)); return <><button className="drawer-scrim" onClick={onClose}/><aside className="detail-drawer"><div className="drawer-head"><div><span>LOG EVENT</span><strong>Event details</strong></div><button className="icon-button" onClick={onClose}><X size={18}/></button></div><div className="drawer-actions"><button onClick={()=>navigator.clipboard.writeText(JSON.stringify(row,null,2))}>Copy JSON</button><button onClick={()=>navigator.clipboard.writeText(value(row,'raw_message'))}>Copy raw</button></div><div className="message-card"><span>MESSAGE</span>{value(row,'_msg')!=='—'?value(row,'_msg'):value(row,'message')}</div><div className="field-heading"><span>FIELDS</span><b>{entries.length}</b></div><div className="field-list">{entries.map(([k,v])=><div key={k}><code>{k}</code><span>{typeof v==='object'?JSON.stringify(v):String(v)}</span><button title="Copy value" onClick={()=>navigator.clipboard.writeText(String(v))}>Copy</button></div>)}</div></aside></> }

function SavedSearches({onOpen}:{onOpen:(q:string)=>void}){const searches=useQuery({queryKey:['saved-searches'],queryFn:({signal})=>api.saved(signal)});return <div className="page"><PageHeader eyebrow="LIBRARY" title="Saved searches" description="Reusable investigations and operational queries."/><section className="panel saved-list">{searches.data?.data.map(s=><div key={s.id}><div><strong>{s.name}</strong><code>{s.query||'*'}</code><span>{s.default_time_range} · {s.created_by}</span></div><button className="secondary-button" onClick={()=>onOpen(s.query)}>Open</button></div>)}{!searches.data?.data.length&&<Empty/>}</section></div>}

function Sources(){return <div className="page"><PageHeader eyebrow="INGEST" title="Sources" description="Configured listeners in the Phase 1 static source registry."/><section className="panel source-table"><div className="source-head"><span>Name</span><span>Protocol</span><span>Container address</span><span>Status</span></div>{[['Syslog UDP','UDP',':1514'],['Syslog TCP','TCP',':1514']].map(s=><div className="source-row" key={s[1]}><span><div className="source-icon"><Server size={16}/></div><strong>{s[0]}</strong></span><code>{s[1]}</code><code>{s[2]}</code><em><i/> Running</em></div>)}</section><Notice>Runtime source management is scheduled for Phase 5. Edit <code>config/syslogx.yaml</code> and restart during Phase 1.</Notice></div>}

function SystemView({ready}:{ready?:{status:string;checks:Record<string,unknown>}}){return <div className="page"><PageHeader eyebrow="SYSTEM" title="Health & readiness" description="Current service and dependency state."/><div className="system-grid"><section className="panel"><PanelTitle title="Readiness checks" subtitle="Refreshed every five seconds"/>{Object.entries(ready?.checks??{}).map(([k,v])=><div className="check-row" key={k}><span><i className={String(v).includes('false')||String(v).includes('unavailable')?'bad':''}/>{k.replace('_',' ')}</span><code>{typeof v==='object'?JSON.stringify(v):String(v)}</code></div>)}</section><section className="panel"><PanelTitle title="Storage" subtitle="Phase 1 backend"/><div className="storage-card"><HardDrive/><div><strong>VictoriaLogs</strong><span>Structured log storage · 30 day retention</span></div></div><div className="status-line"><span>State</span><b>{ready?.status??'checking'}</b></div><div className="status-line"><span>Query window</span><b>15 minutes</b></div></section></div></div>}

function ComingSoon({icon:Icon,title,description}:{icon:typeof Radio;title:string;description:string}){return <div className="page centered"><div className="soon-icon"><Icon/></div><span className="eyebrow">COMING NEXT</span><h1>{title}</h1><p>{description}</p></div>}
function PageHeader({eyebrow,title,description,action}:{eyebrow:string;title:string;description:string;action?:React.ReactNode}){return <div className="page-header"><div><span className="eyebrow">{eyebrow}</span><h1>{title}</h1><p>{description}</p></div>{action}</div>}
function PanelTitle({title,subtitle}:{title:string;subtitle:string}){return <div className="panel-title"><div><strong>{title}</strong><span>{subtitle}</span></div><button className="icon-button"><Menu size={16}/></button></div>}
function Metric({icon:Icon,label,value:metricValue,detail,tone}:{icon:typeof Database;label:string;value:string;detail:string;tone:string}){return <section className="metric"><div className={`metric-icon ${tone}`}><Icon size={18}/></div><div className="metric-label">{label}</div><strong>{metricValue}</strong><span>{detail}</span></section>}
function Notice({children}:{children:React.ReactNode}){return <div className="notice"><Activity size={16}/><div>{children}</div></div>}
function Empty({compact=false}:{compact?:boolean}){return <div className={compact?'empty compact':'empty'}><Filter/><strong>No logs in this window</strong><span>Send a syslog event or adjust your filter.</span></div>}

function deriveStats(rows:LogRow[]){const counts=(key:string)=>{const m=new Map<string,number>();rows.forEach(r=>{const v=value(r,key);if(v!=='—')m.set(v,(m.get(v)??0)+1)});return [...m].map(([name,count])=>({name,count,percent:rows.length?count/rows.length*100:0})).sort((a,b)=>b.count-a.count)};const errors=rows.filter(r=>['error','critical','alert','emergency'].includes(value(r,'severity_name').toLowerCase())).length;const minutes=new Map<string,number>();rows.forEach(r=>{const raw=value(r,'_time')!=='—'?value(r,'_time'):value(r,'timestamp');const d=new Date(raw);if(!Number.isNaN(d.valueOf())){d.setSeconds(0,0);const key=d.toISOString();minutes.set(key,(minutes.get(key)??0)+1)}});const volume=[...minutes].sort(([a],[b])=>a.localeCompare(b)).map(([iso,count])=>({label:new Date(iso).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'}),count}));const sourceSet=new Set(rows.map(r=>value(r,'source_id')).filter(v=>v!=='—'));const times=rows.map(r=>Date.parse(value(r,'_time')!=='—'?value(r,'_time'):value(r,'timestamp'))).filter(Number.isFinite);const seconds=times.length>1?Math.max(1,(Math.max(...times)-Math.min(...times))/1000):1;return {errors,errorPercent:rows.length?errors/rows.length*100:0,sources:sourceSet.size,rate:rows.length/seconds,volume,severities:counts('severity_name'),hosts:counts('hostname')}}
function severityClass(name:string){const n=name.toLowerCase();if(['error','critical','alert','emergency'].includes(n))return'red';if(n==='warning')return'amber';if(n==='notice')return'violet';if(n==='debug')return'gray';return'cyan'}
function formatNumber(n:number){return new Intl.NumberFormat('en-US',{notation:n>9999?'compact':'standard',maximumFractionDigits:1}).format(n)}
function formatTime(raw:string){const d=new Date(raw);return Number.isNaN(d.valueOf())?'—':d.toLocaleTimeString([],{hour12:false,hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3})}
