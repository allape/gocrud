import { describe, expect, test } from "@jest/globals";
import Crudy from "../src/index";

interface IUser {
  id: number;
  name: string;
  deleted: boolean;
  createdAt: string;
  updatedAt: string;
}

describe("test crudy", () => {
  const crudy = new Crudy<IUser>("http://localhost:8080/user");
  test("dry run", async () => {
    await new Promise((r) => setTimeout(r, 1000));

    const u1 = await crudy.save({
      name: "user1",
    });
    expect(u1.id).toBe(1);
    expect(u1.name).toBe("user1");

    const u2 = await crudy.save({
      name: "user2",
    });
    expect(u2.id).toBe(2);
    expect(u2.name).toBe("user2");
    expect(u2.deleted).toBe(false);

    const all = await crudy.all();
    expect(all.length).toBe(2);

    const page = await crudy.page(1, 1);
    expect(page.length).toBe(1);
    expect(page[0].id).toBe(1);

    const deleted = await crudy.delete(1);
    expect(deleted).toBe(true);

    const deletedAgain = await crudy.delete(1);
    expect(deletedAgain).toBe(false);

    const count = await crudy.count<Partial<IUser>>({ deleted: false });
    expect(count).toBe(1);

    const one = await crudy.one(2);
    expect(one.id).toBe(2);
    expect(one.name).toBe("user2");
  });
});
