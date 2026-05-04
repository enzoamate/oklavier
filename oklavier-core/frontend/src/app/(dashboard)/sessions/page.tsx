"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Monitor, Trash2, ExternalLink } from "lucide-react";

const mockSessions = [
  {
    session_id: "abc123",
    image_name: "Google Chrome",
    operational_status: "running",
    start_date: "2026-03-22T01:30:00Z",
    expiration_date: "2026-03-22T02:30:00Z",
    container_ip: "172.31.0.150",
  },
];

export default function SessionsPage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-semibold text-foreground">Sessions actives</h1>
        <p className="text-muted-foreground mt-1">Gérez vos sessions en cours</p>
      </div>

      {mockSessions.length === 0 ? (
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center justify-center py-16">
            <Monitor className="h-12 w-12 text-muted-foreground/40 mb-4" />
            <p className="text-muted-foreground">Aucune session active</p>
            <Button variant="link" className="mt-2 text-oklavier-purple">
              Lancer un espace de travail
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {mockSessions.map((session) => (
            <Card key={session.session_id} className="border border-border">
              <CardContent className="flex items-center justify-between p-6">
                <div className="flex items-center gap-4">
                  <div className="h-12 w-12 rounded-lg bg-gradient-to-br from-oklavier-purple/20 to-oklavier-blue/20 flex items-center justify-center">
                    <Monitor className="h-6 w-6 text-oklavier-purple" />
                  </div>
                  <div>
                    <h3 className="font-medium">{session.image_name}</h3>
                    <p className="text-sm text-muted-foreground">
                      Démarrée {new Date(session.start_date).toLocaleString("fr-FR")}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <Badge className="bg-emerald-100 text-emerald-700 border-0">
                    {session.operational_status}
                  </Badge>
                  <Button size="sm" className="bg-oklavier-blue hover:bg-oklavier-purple">
                    <ExternalLink className="h-3 w-3 mr-1" />
                    Connecter
                  </Button>
                  <Button size="sm" variant="outline" className="text-destructive border-destructive/30 hover:bg-destructive/10">
                    <Trash2 className="h-3 w-3" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
