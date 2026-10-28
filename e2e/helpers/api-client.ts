const API_URL = process.env.API_URL || 'http://localhost:8080/v1';
const BASE_URL = API_URL.replace(/\/v\d+$/, '');

export class ApiClient {
  private token: string = '';

  /**
   * Register a new account via the API.
   */
  async register(accountName: string, adminName: string, email: string, password: string) {
    const res = await fetch(`${API_URL}/accounts`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        account_name: accountName,
        admin_name: adminName,
        admin_email: email,
        password,
      }),
    });
    if (!res.ok) throw new Error(`Register failed: ${res.status} ${await res.text()}`);
    return res.json();
  }

  /**
   * Login with email/password and store the JWT token.
   */
  async login(email: string, password: string): Promise<string> {
    const res = await fetch(`${API_URL}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) throw new Error(`Login failed: ${res.status} ${await res.text()}`);
    const data = await res.json();
    this.token = data.token;
    return this.token;
  }

  /**
   * Create an AI agent.
   */
  async createAgent(name: string, systemPrompt: string, provider: string = 'google') {
    const res = await fetch(`${API_URL}/agents`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify({ name, system_prompt: systemPrompt, provider }),
    });
    if (!res.ok) throw new Error(`Create agent failed: ${res.status} ${await res.text()}`);
    return res.json();
  }

  /**
   * Create a knowledge source for an agent.
   */
  async createKnowledgeSource(name: string, description: string) {
    const res = await fetch(`${API_URL}/knowledge/sources`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify({ name, description }),
    });
    if (!res.ok) throw new Error(`Create knowledge source failed: ${res.status} ${await res.text()}`);
    return res.json();
  }

  /**
   * Add a knowledge chunk to a source.
   */
  async addKnowledgeChunk(sourceId: string, content: string) {
    const res = await fetch(`${API_URL}/knowledge/sources/${sourceId}/chunks`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify({ content }),
    });
    if (!res.ok) throw new Error(`Add chunk failed: ${res.status} ${await res.text()}`);
    return res.json();
  }

  /**
   * Create a channel for an agent.
   */
  async createChannel(agentId: string, provider: string, name: string, credentials: Record<string, string>) {
    const res = await fetch(`${API_URL}/channels`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify({
        agent_id: agentId,
        provider,
        name,
        credentials,
      }),
    });
    if (!res.ok) throw new Error(`Create channel failed: ${res.status} ${await res.text()}`);
    return res.json();
  }

  /**
   * List conversations.
   */
  async listConversations(limit: number = 50, offset: number = 0) {
    const res = await fetch(`${API_URL}/conversations?limit=${limit}&offset=${offset}`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    if (!res.ok) throw new Error(`List conversations failed: ${res.status}`);
    return res.json();
  }

  /**
   * Get account usage.
   */
  async getUsage() {
    const res = await fetch(`${API_URL}/accounts/usage`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    if (!res.ok) throw new Error(`Get usage failed: ${res.status}`);
    return res.json();
  }

  /**
   * Health check.
   */
  async healthCheck(): Promise<boolean> {
    try {
      const res = await fetch(`${BASE_URL}/health`);
      return res.ok;
    } catch {
      return false;
    }
  }

  getToken() {
    return this.token;
  }

  setToken(token: string) {
    this.token = token;
  }
}
