import { Modal } from "antd";
import { IRequestConfig, stringify } from "./fetcheese";
import C, {
  get as getty,
  GetFunc,
  IResponse,
  upload as uploady,
} from "./index";

export async function get<
  T = unknown,
  C extends IRequestConfig<IResponse<T>, T> = IRequestConfig<IResponse<T>, T>,
>(url: string, config?: C): Promise<T> {
  return getty<T>(url, {
    onError: async (e: unknown | Error): Promise<T> => {
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
  getty: GetFunc = get,
): Promise<string> {
  return uploady(url, file, getty);
}

export default class Crudy<T> extends C<T> {
  constructor(public readonly baseUrl: string) {
    super(baseUrl, get);
  }
}
