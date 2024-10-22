import { Modal } from "antd";
import { Fetch, IRequestConfig, stringify } from './fetch';
import C, { get as getee } from "./index";

export async function get<
  T = unknown,
  C extends IRequestConfig = IRequestConfig,
>(url: string, config?: C): Promise<T> {
  return getee<T>(url, {
    onError: async <T>(e: unknown | Error): Promise<T> => {
      return new Promise((resolve, reject) => {
        Modal.confirm({
          title: "Network Error",
          content: `${url}: ${stringify(e)}`,
          okText: "Retry",
          cancelText: "Cancel",
          onOk: () => resolve(get(url, config)),
          onCancel: () => reject(e),
        });
      });
    },
    ...config,
  });
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

export default class Crudy<T> extends C<T> {
  constructor(public readonly baseUrl: string) {
    super(baseUrl, get);
  }
}
