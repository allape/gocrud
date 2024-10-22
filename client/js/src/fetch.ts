export interface IRequestConfig extends RequestInit {
  onHeadersReceived?: (res: Response) => Promise<void> | void;
  onError?: <T>(e: unknown | Error) => Promise<T> | T;
}

export async function fetchee<
  T = unknown,
  C extends IRequestConfig = IRequestConfig,
>(url: string, config?: C): Promise<T> {
  try {
    const res = await fetch(url, config);
    config?.onHeadersReceived?.(res);
    return await res.json();
  } catch (e) {
    if (config?.onError) {
      return config.onError(e);
    }
    throw e;
  }
}

export type Fetch = typeof fetchee;

export function stringify(err: Error | unknown): string {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  return `${err?.message || err?.m || err?.msg || err}`;
}
