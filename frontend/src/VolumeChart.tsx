import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

export default function VolumeChart({data}: {data: Array<{label: string; count: number}>}) {
  return <ResponsiveContainer width="100%" height="100%">
    <AreaChart data={data}>
      <defs><linearGradient id="volume" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#8b5cf6" stopOpacity={.42}/><stop offset="100%" stopColor="#8b5cf6" stopOpacity={0}/></linearGradient></defs>
      <CartesianGrid stroke="#202735" vertical={false}/>
      <XAxis dataKey="label" stroke="#657086" tickLine={false} axisLine={false} fontSize={11}/>
      <YAxis stroke="#657086" tickLine={false} axisLine={false} fontSize={11} width={30}/>
      <Tooltip contentStyle={{background:'#111620',border:'1px solid #2a3342',borderRadius:8,fontSize:12}}/>
      <Area type="monotone" dataKey="count" stroke="#9b7bff" strokeWidth={2} fill="url(#volume)"/>
    </AreaChart>
  </ResponsiveContainer>
}
