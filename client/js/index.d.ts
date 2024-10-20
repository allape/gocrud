export interface IRequestConfig extends RequestInit {
    onHeadersReceived?: (res: Response) => Promise<void> | void;
    onError?: (e: unknown | Error) => Promise<void> | void;
}
export declare function fetchee<T = unknown, C extends IRequestConfig = IRequestConfig>(url: string, config?: C): Promise<T>;
export declare function stringify(err: Error | unknown): string;
export type Code = string;
export type Message = string;
export interface IResponse<Data = unknown> {
    c: Code;
    m: Message;
    d: Data;
}
export declare function get<T = unknown, C extends IRequestConfig = IRequestConfig>(url: string, config?: C): Promise<T>;
export default class Crudy<T> {
    readonly baseUrl: string;
    constructor(baseUrl: string);
    static KeywordsStringify<KEYWORDS = object>(keywords?: KEYWORDS): string;
    all<KEYWORDS = object>(keywords?: KEYWORDS): Promise<T[]>;
    one(id: string | number): Promise<T>;
    page<KEYWORDS = object>(page: number, size: number, keywords?: KEYWORDS): Promise<T[]>;
    count<KEYWORDS = object>(keywords?: KEYWORDS): Promise<number>;
    save(data: Partial<T>): Promise<T>;
    delete(id: string | number): Promise<boolean>;
}
