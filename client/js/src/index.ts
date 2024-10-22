import { Fetch, fetchee, IRequestConfig, stringify } from "./fetch";

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

export function upload(
  url: string,
  file: File | Blob,
  fetch: Fetch = get,
): Promise<string> {
  return fetch<string>(url, {
    method: "POST",
    body: file,
  });
}

export default class Crudy<T> {
  private readonly fetch: Fetch;

  constructor(
    public readonly baseUrl: string,
    fetch?: Fetch,
  ) {
    this.fetch = fetch || get;
  }

  static KeywordsStringify<KEYWORDS = object>(keywords?: KEYWORDS): string {
    return keywords ? `?${new URLSearchParams(keywords)}` : "";
  }

  async all<KEYWORDS = object>(keywords?: KEYWORDS): Promise<T[]> {
    return this.fetch<T[]>(
      `${this.baseUrl}/all${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async one(id: string | number): Promise<T> {
    return this.fetch<T>(`${this.baseUrl}?id=${id}`);
  }

  async page<KEYWORDS = object>(
    page: number,
    size: number,
    keywords?: KEYWORDS,
  ): Promise<T[]> {
    return this.fetch<T[]>(
      `${this.baseUrl}/${page}/${size}${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async count<KEYWORDS = object>(keywords?: KEYWORDS): Promise<number> {
    return this.fetch<number>(
      `${this.baseUrl}/count${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async save(data: Partial<T>): Promise<T> {
    return this.fetch<T>(this.baseUrl, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async delete(id: string | number): Promise<boolean> {
    return this.fetch<boolean>(`${this.baseUrl}?id=${id}`, {
      method: "DELETE",
    });
  }
}
