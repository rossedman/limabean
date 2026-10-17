/** Where this session runs, as the mod sees it. */
export type Place = 'vm' | 'mac' | 'none'

/** A limabean VM's own view of itself, read inside the VM. */
export type InsideVM = {
  name: string
  /** Domains the allowlist lets through; empty when the network is open. */
  allow: string[]
  locked: boolean
  learning: boolean
  /** Packs besides core, from /etc/limabean/mise. */
  packs: string[]
  codeDir: string
}

/** One VM in `limabean ls --json`. */
export type VMRow = {
  name: string
  status: string
  env: string
  cpus: number
  memoryGiB: number
  secrets: number
  sshHost: string
  network: string
  packs: string[] | null
  expires?: string
}

/** One image in `limabean images ls --json`. */
export type ImageRow = {
  name: string
  env: string
  packs: string[]
  built: string
  used: string
  status: string
}

/** One port in `limabean ports --json`. */
export type PortRow = {
  port: number
  process: string
  forwarded: boolean
  url: string
}

/** What the Mac side last read from limabean. */
export type MacView = {
  vms: VMRow[]
  images: ImageRow[]
  ports: Record<string, PortRow[]>
  /** Why the last read failed, if it did. */
  error?: string
  /** Milliseconds since the epoch. */
  readAt: number
}

declare module 'claude-code' {
  interface PluginState {
    limabean: {
      place: Place
      inside: InsideVM | null
      mac: MacView | null
      bandHidden: boolean
    }
  }
}
