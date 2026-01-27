"use client";

import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Trash2 } from "lucide-react";

interface Todo {
  id: number;
  title: string;
  completed: number;
}

interface TodoItemProps {
  todo: Todo;
  onToggle: () => void;
  onDelete: () => void;
  disabled?: boolean;
}

export function TodoItem({ todo, onToggle, onDelete, disabled }: TodoItemProps) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between py-4">
        <div className="flex items-center gap-3">
          <Checkbox
            checked={!!todo.completed}
            onCheckedChange={onToggle}
            disabled={disabled}
          />
          <span
            className={
              todo.completed ? "line-through text-muted-foreground" : ""
            }
          >
            {todo.title}
          </span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={onDelete}
          disabled={disabled}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </CardContent>
    </Card>
  );
}
