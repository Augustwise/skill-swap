export type DashboardExchange = {
  name: string;
  initials: string;
  color: string;
  learning: string;
  completed: number;
  total: number;
  time: string;
};

export type DashboardRequest = {
  name: string;
  initials: string;
  color: string;
  time: string;
  skill: string;
};

// Design fixtures only. The live dashboard stays empty until activity APIs are available.
export const demoExchanges = [
  {
    name: "Марія Гнатюк",
    initials: "МГ",
    color: "pink",
    learning: "Марія: Web-дизайн",
    completed: 3,
    total: 6,
    time: "Сьогодні, 18:00",
  },
  {
    name: "Іван Петренко",
    initials: "ІП",
    color: "green",
    learning: "Іван: Python",
    completed: 1,
    total: 4,
    time: "Пт, 15 бер, 17:00",
  },
] as const;

export const demoRequests = [
  {
    name: "Софія Ткаченко",
    initials: "СТ",
    color: "amber",
    time: "2 год тому",
    skill: "Ілюстрація",
  },
  { name: "Олег Бондар", initials: "ОБ", color: "indigo", time: "вчора", skill: "Python" },
] as const;
