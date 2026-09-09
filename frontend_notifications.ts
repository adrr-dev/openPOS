/**
 * Frontend Integration Helper for openPOS Notification System
 * 
 * Base URL: /api/v1/notifications
 * Authentication: Bearer Token (Authorization: Bearer <token>) required in headers.
 */

export type NotificationType = 'info' | 'warning' | 'alert' | 'low_stock';

export interface NotificationItem {
  id: number;
  created_at: string;
  updated_at: string;
  store_id: number;
  title: string;
  message: string;
  type: NotificationType;
  read: boolean;
  reference_id?: number;
}

export interface NotificationPage {
  items: NotificationItem[];
  total: number;
  page: number;
  limit: number;
}

/**
 * Fetch notifications list.
 * @param token - JWT access token
 * @param page - Page number (optional)
 * @param limit - Items per page (optional)
 * @param unreadOnly - Filter only unread notifications (optional)
 */
export async function fetchNotifications(
  token: string,
  page = 1,
  limit = 20,
  unreadOnly = false
): Promise<NotificationPage> {
  const params = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
    ...(unreadOnly ? { unread: 'true' } : {}),
  });

  const res = await fetch(`/api/v1/notifications?${params.toString()}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: 'application/json',
    },
  });

  if (!res.ok) {
    throw new Error(`Failed to fetch notifications: ${res.statusText}`);
  }

  return res.json();
}

/**
 * Mark a single notification as read.
 */
export async function markNotificationAsRead(token: string, id: number): Promise<void> {
  const res = await fetch(`/api/v1/notifications/${id}/read`, {
    method: 'PATCH',
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: 'application/json',
    },
  });

  if (!res.ok) {
    throw new Error(`Failed to mark notification ${id} as read`);
  }
}

/**
 * Mark all store notifications as read.
 */
export async function markAllNotificationsAsRead(token: string): Promise<void> {
  const res = await fetch('/api/v1/notifications/read-all', {
    method: 'PATCH',
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: 'application/json',
    },
  });

  if (!res.ok) {
    throw new Error('Failed to mark all notifications as read');
  }
}

/**
 * Delete a notification by ID.
 */
export async function deleteNotification(token: string, id: number): Promise<void> {
  const res = await fetch(`/api/v1/notifications/${id}`, {
    method: 'DELETE',
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: 'application/json',
    },
  });

  if (!res.ok) {
    throw new Error(`Failed to delete notification ${id}`);
  }
}

/**
 * Recommended Polling Hook pattern for React / Frontend:
 * 
 * useEffect(() => {
 *   let timer: NodeJS.Timeout;
 *   const poll = async () => {
 *     try {
 *       const data = await fetchNotifications(token, 1, 10, true);
 *       setUnreadCount(data.total);
 *       setNotifications(data.items);
 *     } catch (e) {
 *       console.error(e);
 *     } finally {
 *       timer = setTimeout(poll, 30000); // Poll every 30 seconds
 *     }
 *   };
 *   poll();
 *   return () => clearTimeout(timer);
 * }, [token]);
 */
