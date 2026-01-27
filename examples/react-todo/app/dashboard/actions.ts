"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { requireAuth } from "@/lib/auth";
import { getUserTenant } from "@/lib/db";
import { eq } from "@atomicbase/sdk";
import { deleteSessionCookie, invalidateSession } from "@/lib/session";

export async function addTodo(formData: FormData) {
  const { user } = await requireAuth();
  const title = formData.get("title") as string;

  if (!title?.trim()) {
    throw new Error("Title is required");
  }

  const userTenant = getUserTenant(user.tenantName);
  await userTenant.from("todos").insert({
    title: title.trim(),
    completed: 0,
  });

  revalidatePath("/dashboard");
}

export async function toggleTodo(todoId: number) {
  const { user } = await requireAuth();

  const userTenant = getUserTenant(user.tenantName);

  // Get current state
  const { data: todo } = await userTenant
    .from("todos")
    .select()
    .where(eq("id", todoId))
    .single();

  if (!todo) {
    throw new Error("Todo not found");
  }

  // Toggle completed state
  await userTenant
    .from("todos")
    .update({
      completed: todo.completed ? 0 : 1,
      updated_at: new Date().toISOString(),
    })
    .where(eq("id", todoId));

  revalidatePath("/dashboard");
}

export async function deleteTodo(todoId: number) {
  const { user } = await requireAuth();

  const userTenant = getUserTenant(user.tenantName);
  await userTenant.from("todos").delete().where(eq("id", todoId));

  revalidatePath("/dashboard");
}

export async function logout() {
  const { session } = await requireAuth();

  await invalidateSession(session.id);
  await deleteSessionCookie();

  redirect("/");
}
