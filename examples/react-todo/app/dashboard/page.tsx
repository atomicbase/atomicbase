import { redirect } from "next/navigation";
import { requireAuth } from "@/lib/auth";
import { getUserTenant } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TodoList } from "@/components/todo-list";
import { AddTodoForm } from "@/components/add-todo-form";
import { LogoutButton } from "@/components/logout-button";

interface Todo {
  id: number;
  title: string;
  completed: number;
  created_at: string;
  updated_at: string;
}

export default async function DashboardPage() {
  let user;
  try {
    const auth = await requireAuth();
    user = auth.user;
  } catch {
    redirect("/");
  }

  // Fetch todos from user's tenant database
  const userTenant = getUserTenant(user.tenantName);
  const { data: todos } = await userTenant
    .from<Todo>("todos")
    .select()
    .orderBy("created_at", "desc");

  return (
    <div className="min-h-screen bg-zinc-50 dark:bg-zinc-950 p-8">
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold">My Todos</h1>
            <p className="text-muted-foreground">Welcome, {user.name}</p>
          </div>
          <LogoutButton />
        </div>

        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Add New Todo</CardTitle>
          </CardHeader>
          <CardContent>
            <AddTodoForm />
          </CardContent>
        </Card>

        <TodoList initialTodos={todos ?? []} />
      </div>
    </div>
  );
}
