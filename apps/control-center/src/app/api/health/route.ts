export const dynamic = "force-dynamic";

export function GET() {
  return Response.json(
    {
      status: "healthy",
      service: "itemba-z-control-center",
      timestamp: new Date().toISOString(),
    },
    {
      headers: {
        "Cache-Control": "no-store",
      },
    },
  );
}
