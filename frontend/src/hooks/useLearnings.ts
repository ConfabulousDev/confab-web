// ABOUTME: React Query hooks for learnings CRUD operations
// ABOUTME: Provides useLearnings, useCreateLearning, useUpdateLearning, useDeleteLearning
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { learningsAPI } from "@/services/api";

export function useLearnings(filters?: Record<string, string>) {
  return useQuery({
    queryKey: ["learnings", filters],
    queryFn: () => learningsAPI.list(filters),
    staleTime: 30 * 1000,
  });
}

export function useCreateLearning() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: learningsAPI.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["learnings"] });
    },
  });
}

export function useUpdateLearning() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...data
    }: {
      id: string;
      title?: string;
      body?: string;
      tags?: string[];
      status?: string;
    }) => learningsAPI.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["learnings"] });
    },
  });
}

export function useDeleteLearning() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: learningsAPI.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["learnings"] });
    },
  });
}

export function useExportLearning() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: learningsAPI.export,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["learnings"] });
    },
  });
}
