"use client";

import { useOptimistic, useTransition } from "react";
import { toast } from "sonner";
import { TodoItem } from "./todo-item";
import { toggleTodo, deleteTodo } from "@/app/dashboard/actions";

interface Todo {
  id: number;
  title: string;
  completed: number;
  created_at: string;
  updated_at: string;
}

interface TodoListProps {
  initialTodos: Todo[];
}

export function TodoList({ initialTodos }: TodoListProps) {
  const [isPending, startTransition] = useTransition();
  const [optimisticTodos, updateOptimisticTodos] = useOptimistic(
    initialTodos,
    (state, update: { type: "toggle" | "delete"; id: number }) => {
      if (update.type === "delete") {
        return state.filter((t) => t.id !== update.id);
      }
      return state.map((t) =>
        t.id === update.id ? { ...t, completed: t.completed ? 0 : 1 } : t
      );
    }
  );

  const handleToggle = (id: number) => {
    startTransition(async () => {
      updateOptimisticTodos({ type: "toggle", id });
      const result = await toggleTodo(id);
      if (result.error) {
        toast.error(result.error);
      }
    });
  };

  const handleDelete = (id: number) => {
    startTransition(async () => {
      updateOptimisticTodos({ type: "delete", id });
      const result = await deleteTodo(id);
      if (result.error) {
        toast.error(result.error);
      }
    });
  };

  if (optimisticTodos.length === 0) {
    return (
      <p className="text-center text-muted-foreground py-8">
        No todos yet. Add one above!
      </p>
    );
  }

  return (
    <div className="space-y-2">
      {optimisticTodos.map((todo) => (
        <TodoItem
          key={todo.id}
          todo={todo}
          onToggle={() => handleToggle(todo.id)}
          onDelete={() => handleDelete(todo.id)}
          disabled={isPending}
        />
      ))}
    </div>
  );
}
