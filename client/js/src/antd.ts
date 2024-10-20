import { Modal } from "antd";
import C, { get as getee, IRequestConfig, stringify } from "./index";

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

export default class Crudy<T> extends C<T> {
  constructor(public readonly baseUrl: string) {
    super(baseUrl, get);
  }
}
