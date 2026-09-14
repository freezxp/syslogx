export type LogRow = Record<string, unknown> & { _time?:string; timestamp?:string; _msg?:string; message?:string; hostname?:string; severity_name?:string; app_name?:string }
export type Filter = { field:string; operator:string; value?:string }
export type LogQuery = { start:string; end:string; query?:string; native_query?:string; filters?:Filter[]; limit:number; cursor?:string }
export type SearchResponse = { data:LogRow[]; next_cursor?:string; truncated:boolean }
export type ValueCount = { value:string; count:number }
export type Stats = { total:number; volume:{timestamp:string;count:number}[]; severities:ValueCount[]; top_hosts:ValueCount[]; top_applications:ValueCount[] }
export type ReadyResponse = { status:'ready'|'not_ready'; checks:Record<string,unknown> }
export type SavedSearch = { id:string; name:string; description:string; query:string; default_time_range:string; created_by:string; created_at:string; updated_at:string }

async function request<T>(path:string, init:RequestInit={}):Promise<T> {
  const response=await fetch(path,{credentials:'same-origin',...init,headers:{Accept:'application/json',...(init.body?{'Content-Type':'application/json'}:{}),...init.headers}})
  if(!response.ok){let message=`${response.status} ${response.statusText}`;try{const b=await response.json() as {message?:string};message=b.message??message}catch{}throw new Error(message)}
  if(response.status===204)return undefined as T
  return response.json() as Promise<T>
}
const post=<T>(path:string,body:unknown,signal?:AbortSignal)=>request<T>(path,{method:'POST',body:JSON.stringify(body),signal})

export const api={
  me:(signal?:AbortSignal)=>request<{username:string;role:string;auth_enabled:boolean}>('/api/v1/auth/me',{signal}),
  login:(username:string,password:string)=>post<{username:string;role:string}>('/api/v1/auth/login',{username,password}),
  search:(q:LogQuery,signal?:AbortSignal)=>post<SearchResponse>('/api/v1/logs/search',q,signal),
  stats:(q:LogQuery,signal?:AbortSignal)=>post<Stats>('/api/v1/logs/stats',q,signal),
  fields:(q:LogQuery,signal?:AbortSignal)=>post<{data:{name:string;count:number}[]}>('/api/v1/fields',q,signal),
  values:(field:string,q:LogQuery,signal?:AbortSignal)=>post<{data:ValueCount[]}>(`/api/v1/fields/${encodeURIComponent(field)}/values`,q,signal),
  ready:async(signal?:AbortSignal)=>{const response=await fetch('/ready',{signal});if(response.status!==200&&response.status!==503)throw new Error(response.statusText);return response.json() as Promise<ReadyResponse>},
  saved:(signal?:AbortSignal)=>request<{data:SavedSearch[]}>('/api/v1/saved-searches',{signal}),
  save:(v:Pick<SavedSearch,'name'|'description'|'query'|'default_time_range'>)=>post<SavedSearch>('/api/v1/saved-searches',v),
  removeSaved:(id:string)=>request<void>(`/api/v1/saved-searches/${id}`,{method:'DELETE'})
}
export const value=(row:LogRow,key:string)=>{const raw=row[key];if(raw===null||raw===undefined)return'—';return typeof raw==='object'?JSON.stringify(raw):String(raw)}
export function range(duration:string){const end=new Date();const units:Record<string,number>={m:60000,h:3600000,d:86400000};const match=duration.match(/^(\d+)([mhd])$/);const ms=match?Number(match[1])*units[match[2]]:3600000;return{start:new Date(end.getTime()-ms).toISOString(),end:end.toISOString()}}
