"use client";

import { useRef } from "react";
import { useFormStatus } from "react-dom";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { addTodo } from "@/app/dashboard/actions";

function SubmitButton() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" disabled={pending}>
      {pending ? "Adding..." : "Add Todo"}
    </Button>
  );
}

export function AddTodoForm() {
  const formRef = useRef<HTMLFormElement>(null);

  async function handleSubmit(formData: FormData) {
    const result = await addTodo(formData);
    if (result.error) {
      toast.error(result.error);
      return;
    }
    formRef.current?.reset();
  }

  return (
    <form ref={formRef} action={handleSubmit} className="flex gap-2">
      <Input
        name="title"
        placeholder="What needs to be done?"
        required
        className="flex-1"
      />
      <SubmitButton />
    </form>
  );
}
