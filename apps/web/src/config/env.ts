import { z } from "zod";

const browserEnvironmentSchema = z.object({
  VITE_API_BASE_URL: z
    .string()
    .trim()
    .default("")
    .refine(
      (value) => value === "" || value.startsWith("http://") || value.startsWith("https://"),
      {
        message: "VITE_API_BASE_URL must be empty or an absolute HTTP(S) URL",
      },
    ),
});

const result = browserEnvironmentSchema.safeParse({
  VITE_API_BASE_URL: import.meta.env.VITE_API_BASE_URL,
});

if (!result.success) {
  throw new Error(`Invalid frontend configuration: ${result.error.message}`);
}

export const environment = Object.freeze({
  apiBaseUrl: result.data.VITE_API_BASE_URL.replace(/\/+$/, ""),
});
