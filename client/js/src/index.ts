import { fetcheese, IRequestConfig, stringify } from "./fetcheese";

export type Code = string;
export type Message = string;

export interface IResponse<Data = unknown> {
  c: Code;
  m: Message;
  d: Data;
}

export async function get<
  T = unknown,
  C extends IRequestConfig<IResponse<T>, T> = IRequestConfig<IResponse<T>, T>,
>(url: string, config?: C): Promise<T> {
  return await fetcheese<IResponse<T>, T>(url, {
    onDataReceived: (data: IResponse<T>): T => {
      if (data.c !== "0") {
        throw data;
      }
      return data.d;
    },
    onError: async (e: unknown | Error): Promise<T> => {
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
}

export type GetFunc = typeof get;

export function upload(
  url: string,
  file: File | Blob,
  getty: GetFunc = get,
): Promise<string> {
  return getty<string>(url, {
    method: "POST",
    body: file,
  });
}

export default class Crudy<T> {
  constructor(
    public readonly baseUrl: string,
    private readonly getty: GetFunc = get,
  ) {}

  static KeywordsStringify<KEYWORDS = object>(keywords?: KEYWORDS): string {
    if (!keywords) {
      return "";
    }
    Object.entries(keywords).forEach(([key, value]) => {
      if (value === undefined) {
        delete (keywords as Record<string, unknown>)[key];
      }
    });
    return `?${new URLSearchParams(keywords)}`;
  }

  async all<KEYWORDS = object>(keywords?: KEYWORDS): Promise<T[]> {
    return this.getty<T[]>(
      `${this.baseUrl}/all${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async one(id: string | number): Promise<T> {
    return this.getty<T>(`${this.baseUrl}?id=${id}`);
  }

  async page<KEYWORDS = object>(
    page: number,
    size: number,
    keywords?: KEYWORDS,
  ): Promise<T[]> {
    return this.getty<T[]>(
      `${this.baseUrl}/${page}/${size}${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async count<KEYWORDS = object>(keywords?: KEYWORDS): Promise<number> {
    return this.getty<number>(
      `${this.baseUrl}/count${Crudy.KeywordsStringify(keywords)}`,
    );
  }

  async save(data: Partial<T>): Promise<T> {
    return this.getty<T>(this.baseUrl, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async delete(id: string | number): Promise<boolean> {
    return this.getty<boolean>(`${this.baseUrl}?id=${id}`, {
      method: "DELETE",
    });
  }
}
