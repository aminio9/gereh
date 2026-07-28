import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../app/AppProviders";
import { HomePage } from "./HomePage";

describe("HomePage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows the API status when the backend is healthy", async () => {
    const fetchMock = vi.fn(() => {
      return new Response(
        JSON.stringify({
          status: "ok",
          service: "api-bff",
          version: "dev",
        }),
        {
          status: 200,
          headers: {
            "content-type": "application/json",
            "x-request-id": "test-request-id",
          },
        },
      );
    });

    vi.stubGlobal("fetch", fetchMock);

    render(
      <AppProviders>
        <HomePage />
      </AppProviders>,
    );

    expect(await screen.findByText("API BFF is healthy.")).toBeInTheDocument();

    expect(screen.getByText("api-bff · dev")).toBeInTheDocument();

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/v1\/status$/),
      expect.objectContaining({
        credentials: "include",
        method: "GET",
      }),
    );
  });
});
