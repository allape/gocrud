import { describe, test } from "@jest/globals";
import { upload } from "../src";

describe("test upload", () => {
  const context = `1234abcd_${Math.random()}`;
  const fullFilename = "/js1/test1.txt";
  const blob = new Blob([context], { type: "text/plain" });
  test("post", async () => {
    const savedPath = await upload(
      `http://localhost:8080/static${fullFilename}`,
      blob,
    );
    expect(savedPath).toBe(fullFilename);

    const res = await fetch(`http://localhost:8080/static/${fullFilename}`);
    const text = await res.text();
    expect(text).toBe(context);
  });
});
