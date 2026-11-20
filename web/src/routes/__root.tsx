import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { Layout } from "@/components/Layout";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <Layout>
      <Outlet />
    </Layout>
  ),
  notFoundComponent: () => <p className="text-muted-foreground">Page not found.</p>,
});
