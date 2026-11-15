const GIB = 2 ** 30;
const MIB = 2 ** 20;

/** Quota is binary: GiB = 2^30 bytes, MiB = 2^20 bytes. */
export function bytes(n: number): string {
  const abs = Math.abs(n);
  if (abs >= GIB) return `${(n / GIB).toFixed(2)} GiB`;
  if (abs >= MIB) return `${(n / MIB).toFixed(1)} MiB`;
  return `${Math.round(n / 1024)} KiB`;
}

const inr = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR", minimumFractionDigits: 2 });

/** Money arrives as integer paise. */
export function rupees(paise: number): string {
  return inr.format(paise / 100);
}

export function km(metres: number): string {
  return metres >= 1000 ? `${(metres / 1000).toFixed(1)} km` : `${Math.round(metres)} m`;
}

export function minutes(seconds: number): string {
  return `${Math.max(1, Math.round(seconds / 60))} min`;
}

export function when(iso: string): string {
  return new Date(iso).toLocaleString("en-IN", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

export const STATUS_LABEL: Record<string, string> = {
  requested: "Finding a cab",
  assigned: "Cab on the way",
  in_progress: "On trip",
  completed: "Completed",
  cancelled: "Cancelled",
};
