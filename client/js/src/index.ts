// region fetch wrapper

export interface IRequestConfig extends RequestInit {
  onHeadersReceived?: (res: Response) => Promise<void> | void;
  onError?: (e: unknown | Error) => Promise<void> | void;
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
      config.onError(e);
    }
    throw e;
  }
}

// endregion

export function stringify(err: Error | unknown): string {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  return `${err?.message || err?.m || err?.msg || err}`;
}

export type Code = string;
export type Message = string;

export interface IResponse<Data = unknown> {
  c: Code;
  m: Message;
  d: Data;
}

export async function get<
  T = unknown,
  C extends IRequestConfig = IRequestConfig,
>(url: string, config?: C): Promise<T> {
  const res = await fetchee<IResponse<T>>(url, {
    onError: async <T>(e: unknown | Error): Promise<T> => {
      if (confirm(`${stringify(e)} | Retry?`)) {
        return get(url, config);
      } else {
        throw e;
      }
    },
    onHeadersReceived: (res: Response): void => {
      if (res.status < 200 || res.status >= 300) {
        throw new Error(res.statusText);
      }
    },
    ...config,
  });

  if (res.c !== "0") {
    throw res;
  }

  return res.d;
}

export default class Crudy<T> {
  constructor(public readonly baseUrl: string) {}

  static KeywordsStringify<KEYWORDS = object>(keywords?: KEYWORDS): string {
    return keywords ? `?${new URLSearchParams(keywords)}` : "";
  }

  async all<KEYWORDS = object>(keywords?: KEYWORDS): Promise<T[]> {
    return get<T[]>(`${this.baseUrl}/all${Crudy.KeywordsStringify(keywords)}`);
  }

  async one(id: string | number): Promise<T> {
    return get<T>(`${this.baseUrl}?id=${id}`);
  }

  async page<KEYWORDS = object>(
    page: number,
    size: number,
    keywords?: KEYWORDS,
  ): Promise<T[]> {
    return get<T[]>(
      `${this.baseUrl}/${page}/${size}${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async count<KEYWORDS = object>(keywords?: KEYWORDS): Promise<number> {
    return get<number>(
      `${this.baseUrl}/count${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async save(data: Partial<T>): Promise<T> {
    return get<T>(this.baseUrl, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async delete(id: string | number): Promise<boolean> {
    return get<boolean>(`${this.baseUrl}?id=${id}`, {
      method: "DELETE",
    });
  }
}
