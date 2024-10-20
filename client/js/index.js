"use strict";
// region fetch wrapper
Object.defineProperty(exports, "__esModule", { value: true });
exports.fetchee = fetchee;
exports.stringify = stringify;
exports.get = get;
async function fetchee(url, config) {
    try {
        const res = await fetch(url, config);
        config?.onHeadersReceived?.(res);
        return await res.json();
    }
    catch (e) {
        if (config?.onError) {
            config.onError(e);
        }
        throw e;
    }
}
// endregion
function stringify(err) {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    return `${err?.message || err?.m || err?.msg || err}`;
}
async function get(url, config) {
    const res = await fetchee(url, {
        onError: async (e) => {
            if (confirm(`${stringify(e)} | Retry?`)) {
                return get(url, config);
            }
            else {
                throw e;
            }
        },
        onHeadersReceived: (res) => {
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
class Crudy {
    constructor(baseUrl) {
        this.baseUrl = baseUrl;
    }
    static KeywordsStringify(keywords) {
        return keywords ? `?${new URLSearchParams(keywords)}` : "";
    }
    async all(keywords) {
        return get(`${this.baseUrl}/all${Crudy.KeywordsStringify(keywords)}`);
    }
    async one(id) {
        return get(`${this.baseUrl}?id=${id}`);
    }
    async page(page, size, keywords) {
        return get(`${this.baseUrl}/${page}/${size}${Crudy.KeywordsStringify(keywords)}`);
    }
    async count(keywords) {
        return get(`${this.baseUrl}/count${Crudy.KeywordsStringify(keywords)}`);
    }
    async save(data) {
        return get(this.baseUrl, {
            method: "PUT",
            body: JSON.stringify(data),
        });
    }
    async delete(id) {
        return get(`${this.baseUrl}?id=${id}`, {
            method: "DELETE",
        });
    }
}
exports.default = Crudy;
