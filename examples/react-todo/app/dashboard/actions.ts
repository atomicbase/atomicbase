"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { requireAuth } from "@/lib/auth";
import { getUserDatabase } from "@/lib/db";
import { eq } from "@atomicbase/sdk";
import { deleteSessionCookie, invalidateSession } from "@/lib/session";

export type ActionResult = { error?: string };

export async function addTodo(formData: FormData): Promise<ActionResult> {
  const { user } = await requireAuth();
  const title = formData.get("title") as string;

  if (!title?.trim()) {
    return { error: "Title is required" };
  }

  const userDatabase = getUserDatabase(user.tenantName);
  const { error } = await userDatabase.from("todos").insert({
    title: title.trim(),
    completed: 0,
  });

  if (error) {
    return { error: error.message };
  }

  revalidatePath("/dashboard");
  return {};
}

export async function toggleTodo(todoId: number): Promise<ActionResult> {
  const { user } = await requireAuth();

  const userDatabase = getUserDatabase(user.tenantName);

  // Get current state
  const { data: todo, error: fetchError } = await userDatabase
    .from("todos")
    .select()
    .where(eq("id", todoId))
    .single();

  if (fetchError) {
    return { error: fetchError.message };
  }

  if (!todo) {
    return { error: "Todo not found" };
  }

  // Toggle completed state
  const { error: updateError } = await userDatabase
    .from("todos")
    .update({
      completed: todo.completed ? 0 : 1,
      updated_at: new Date().toISOString(),
    })
    .where(eq("id", todoId));

  if (updateError) {
    return { error: updateError.message };
  }

  revalidatePath("/dashboard");
  return {};
}

export async function deleteTodo(todoId: number): Promise<ActionResult> {
  const { user } = await requireAuth();

  const userDatabase = getUserDatabase(user.tenantName);
  const { error } = await userDatabase.from("todos").delete().where(eq("id", todoId));

  if (error) {
    return { error: error.message };
  }

  revalidatePath("/dashboard");
  return {};
}

export async function logout() {
  const { session } = await requireAuth();

  await invalidateSession(session.id);
  await deleteSessionCookie();

  redirect("/");
}
