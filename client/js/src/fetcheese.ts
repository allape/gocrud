export interface IRequestConfig<T = unknown, D = T> extends RequestInit {
  onHeadersReceived?: (res: Response) => Promise<void> | void;
  onDataReceived?: (data: T) => Promise<D> | D;
  onError?: (e: unknown | Error) => Promise<D> | D;
}

export async function fetcheese<
  T = unknown,
  D = T,
  C extends IRequestConfig<T, D> = IRequestConfig<T, D>,
>(url: string, config?: C): Promise<D> {
  try {
    const res = await fetch(url, config);
    config?.onHeadersReceived?.(res);
    const data = await res.json();
    return config?.onDataReceived ? await config.onDataReceived(data) : data;
  } catch (e) {
    if (config?.onError) {
      return config.onError(e);
    }
    throw e;
  }
}

export function stringify(err: Error | unknown): string {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  return `${err?.message || err?.m || err?.msg || err}`;
}
