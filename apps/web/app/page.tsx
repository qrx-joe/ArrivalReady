import { redirect } from "next/navigation";

// The operator workflow starts at the project list; `/` redirects there.
export default function Home() {
  redirect("/dashboard");
}
